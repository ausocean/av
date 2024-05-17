/*
NAME
  lex.go

AUTHOR
  Trek Hopton <trek@ausocean.org>

LICENSE
  Copyright (C) 2024 the Australian Ocean Lab (AusOcean). All Rights Reserved. 

  The Software and all intellectual property rights associated
  therewith, including but not limited to copyrights, trademarks,
  patents, and trade secrets, are and will remain the exclusive
  property of the Australian Ocean Lab (AusOcean).
*/


package codecutil

import (
	"errors"
	"fmt"
	"io"
	"math"
	"time"
	"context"
)

// ByteLexer is used to lex bytes using a buffer size which is configured upon construction.
type ByteLexer struct {
	bufSize int
}

// NewByteLexer returns a pointer to a ByteLexer with the given buffer size.
func NewByteLexer(s int) (*ByteLexer, error) {
	if s <= 0 {
		return nil, fmt.Errorf("invalid buffer size: %v", s)
	}
	return &ByteLexer{bufSize: s}, nil
}

// zeroTicks can be used to create an instant ticker.
var zeroTicks chan time.Time

func init() {
	zeroTicks = make(chan time.Time)
	close(zeroTicks)
}

// Lex reads l.bufSize bytes from src and writes them to dst every d seconds.
func (l *ByteLexer) Lex(dst io.Writer, src io.Reader, d time.Duration) error {
	if d < 0 {
		return fmt.Errorf("invalid delay: %v", d)
	}

	var ticker *time.Ticker
	if d == 0 {
		ticker = &time.Ticker{C: zeroTicks}
	} else {
		ticker = time.NewTicker(d)
		defer ticker.Stop()
	}

	buf := make([]byte, l.bufSize)
	for {
		<-ticker.C
		off, err := src.Read(buf)
		// The only error that will stop the lexer is an EOF.
		if err == io.EOF {
			return err
		} else if err != nil {
			continue
		}
		_, err = dst.Write(buf[:off])
		if err != nil {
			return err
		}
	}
}

var errBuffChanReceiveTimeout = errors.New("buffer chan receive timeout")

// Noop reads media "frames" from src, queues and then writes to dst at intervals,
// maintaining a steady number of frames stored in the queue (channel). This ensures frames
// are outputted at a consistent rate; useful if reads occur from src in blocks (a
// side effect if src is connected to an input that receives packets containing
// multiple frames at intervals e.g. MPEG-TS over HTTP).
// Noop assumes that writing to the input connected to src is blocked until the
// entire previous write is read, i.e. src is expected to be connected to
// a pipe-like structure.
func Noop(dst io.Writer, src io.Reader, d time.Duration) error {
	// Controller tuning constants.
	const (
		target        = 500                   // Target channel size to maintain.
		coef          = 0.05                  // Proportional controller coefficient.
		minDelay      = 1                     // Minimum delay between writes (micro s).
		maxDelay      = 1000000               // Maximum delay between writes (micro s).
		defaultDelay  = 40 * time.Millisecond // Default delay between writes, equivalent to ~25fps.
		bufferTimeout = 1 * time.Minute       // Buffer read/write timeout.
	)

	// Ring buffer tuning.
	const (
		ringCap      = 1000   // Ring buffer capacity.
		ringElemSize = 250000 // Ring buffer element size i.e. max h264 frame size.
	)

	if d < 0 {
		return fmt.Errorf("invalid delay: %v", d)
	}

	if d == 0 {
		d = defaultDelay
	}

	var (
		delay = time.NewTicker(d)                                   // Ticker based on delay between frames.
		errCh = make(chan error)                                    // Used by the output routine to signal errors to the main loop.
		rb    = newRingBuffer(ringElemSize, ringCap, bufferTimeout) // Use a ring buffer to reduce allocation and GC load.
	)
	defer delay.Stop()

	// This routine is responsible for frame output to rest of the pipeline.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		for {
			err := rb.writeTo(dst)
			if err != nil && !errors.Is(err, errBuffChanReceiveTimeout) {
				errCh <- fmt.Errorf("could not write to dst: %w", err)
			}

			select {
			case <-delay.C:
			case <-ctx.Done():
				return
			}

			// Adjust delay using proportional controller.
			adj := coef * float64(target-rb.len())
			dMicro := float64(d.Microseconds()) + adj
			d = time.Microsecond * time.Duration(math.Min(math.Max(dMicro, minDelay), maxDelay))
			delay.Reset(d)
		}
	}()

	// This loop is responsible for reading frames and checking any errors from
	// the output routine.
	for {
		err := rb.readFrom(src)
		if err != nil {
			return fmt.Errorf("could not read src: %w", err)
		}
		select {
		case err := <-errCh:
			return fmt.Errorf("error from output routine: %w", err)
		default:
		}
	}
}

// ringBuffer is a basic concurrency safe ring buffer. Concurrency safety is
// achieved using a channel between read and write methods i.e. overwrite/dropping
// behaviour is absent and blocking will occur.
type ringBuffer struct {
	n       int           // Num. of elements.
	i       int           // Current index in underlying buffer.
	buf     [][]byte      // Underlying buffer.
	ch      chan []byte   // ch will act as our concurrency safe queue.
	timeout time.Duration // readFrom and writeTo timeout.
}

// newRingBuffer returns a new ringBuffer with sz as the element size in bytes
// and cap as the number of elements.
func newRingBuffer(sz, cap int, timeout time.Duration) *ringBuffer {
	rb := &ringBuffer{
		buf:     make([][]byte, cap),
		n:       cap,
		ch:      make(chan []byte, cap),
		timeout: timeout,
	}
	for i := range rb.buf {
		rb.buf[i] = make([]byte, sz)
	}
	return rb
}

// readFrom gets the next []byte from the buffer and uses it to read from r.
// This data is then stored in the buffer channel ready for writeTo to retreive.
// readFrom will block if the buffer channel is filled, at least within the
// timeout, otherwise an error is returned.
func (b *ringBuffer) readFrom(r io.Reader) error {
	buf := b.buf[b.i]
	b.i++
	if b.i == b.n {
		b.i = 0
	}
	n, err := r.Read(buf)
	if err != nil {
		return err
	}
	timeout := time.NewTimer(b.timeout)
	select {
	case b.ch <- buf[:n]:
	case <-timeout.C:
		return errors.New("buffer chan send timeout")
	}
	return nil
}

// writeTo tries to get a []byte from the buffer channel within the timeout
// and then writes to w if successful, otherwise an error is returned.
func (b *ringBuffer) writeTo(w io.Writer) error {
	timeout := time.NewTimer(b.timeout)
	select {
	case p := <-b.ch:
		_, err := w.Write(p)
		if err != nil {
			return err
		}
	case <-timeout.C:
		return errBuffChanReceiveTimeout
	}
	return nil
}

func (b *ringBuffer) len() int {
	return len(b.ch)
}
