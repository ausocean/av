/*
NAME
  senders.go

DESCRIPTION
  See Readme.md

AUTHORS
  Saxon A. Nelson-Milton <saxon@ausocean.org>
  Alan Noble <alan@ausocean.org>

LICENSE
  Copyright (C) 2024 the Australian Ocean Lab (AusOcean). All Rights Reserved. 

  The Software and all intellectual property rights associated
  therewith, including but not limited to copyrights, trademarks,
  patents, and trade secrets, are and will remain the exclusive
  property of the Australian Ocean Lab (AusOcean).
*/


package revid

import (
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"sync"
	"syscall"
	"time"

	"github.com/Comcast/gots/v2/packet"

	"github.com/ausocean/av/container/mts"
	"github.com/ausocean/av/protocol/rtmp"
	"github.com/ausocean/av/protocol/rtp"
	"github.com/ausocean/client/pi/netsender"
	"github.com/ausocean/utils/logging"
	"github.com/ausocean/utils/pool"
)

// Sender pool buffer read timeouts.
const (
	rtmpPoolReadTimeout   = 1 * time.Second
	mtsPoolReadTimeout    = 1 * time.Second
	mtsBufferPoolMaxAlloc = 5 << 20 // 5MiB.
	maxBuffLen            = 50000000
)

var (
	adjustedRTMPPoolElementSize int
	adjustedMTSPoolElementSize  int
)

var errReportCallbackNil = errors.New("report callback is nil")

type httpSenderOption func(s *httpSender) error

// withReportCallback provides a functional option to set the report callback
// function. This can be used to record the number of bytes sent.
func withReportCallback(report func(sent int)) httpSenderOption {
	return func(s *httpSender) error {
		if report == nil {
			return errReportCallbackNil
		}
		s.report = report
		return nil
	}
}

// withHTTPAddress provides a functional option to set the destination http
// address.
func withHTTPAddress(addr string) httpSenderOption {
	return func(s *httpSender) error {
		s.addr = addr
		return nil
	}
}

// httpSender provides an implemntation of io.Writer to perform sends to a http
// destination.
type httpSender struct {
	client *netsender.Sender
	log    logging.Logger
	report func(sent int)
	addr   string
}

// newHttpSender returns a pointer to a new httpSender.
// report is callback that can be used to report the amount of data sent per write.
// This can be set to nil.
func newHTTPSender(ns *netsender.Sender, log logging.Logger, opts ...httpSenderOption) (*httpSender, error) {
	s := &httpSender{client: ns, log: log}
	for _, opt := range opts {
		err := opt(s)
		if err != nil {
			return nil, err
		}
	}
	return s, nil
}

// Write implements io.Writer.
func (s *httpSender) Write(d []byte) (int, error) {
	s.log.Debug("HTTP sending", "address", s.addr)
	err := httpSend(d, s.client, s.log, s.addr)
	if err == nil {
		s.log.Debug("good send", "len", len(d))
		if s.report != nil {
			s.report(len(d))
		}
	} else {
		s.log.Debug("bad send", "error", err)
	}
	return len(d), err
}

func (s *httpSender) Close() error { return nil }

func httpSend(d []byte, client *netsender.Sender, log logging.Logger, addr string) error {
	// Only send if "V0" or "S0" are configured as input.
	send := false
	ip := client.Param("ip")
	log.Debug("making pins, and sending mts request", "ip", ip)
	pins := netsender.MakePins(ip, "V,S")
	for i, pin := range pins {
		switch pin.Name {
		case "V0":
			pins[i].MimeType = "video/mp2t"
		case "S0":
			pins[i].MimeType = "audio/x-wav"
		default:
			continue
		}
		pins[i].Value = len(d)
		pins[i].Data = d
		send = true
		break
	}

	if !send {
		return nil
	}
	reply, _, err := client.Send(netsender.RequestMts, pins, netsender.WithMtsAddress(addr))
	if err != nil {
		return err
	}
	log.Debug("good request", "reply", reply)
	return extractMeta(reply, log)
}

// extractMeta looks at a reply at extracts any time or location data - then used
// to update time and location information in the mpegts encoder.
func extractMeta(r string, log logging.Logger) error {
	dec, err := netsender.NewJSONDecoder(r)
	if err != nil {
		return nil
	}
	// Extract time from reply if mts.Realtime has not been set.
	if !mts.RealTime.IsSet() {
		t, err := dec.Int("ts")
		if err != nil {
			log.Warning("No timestamp in reply")
		} else {
			log.Debug("got timestamp", "ts", t)
			mts.RealTime.Set(time.Unix(int64(t), 0))
		}
	}

	// Extract location from reply
	g, err := dec.String("ll")
	if err != nil {
		log.Debug("No location in reply")
	} else {
		log.Debug(fmt.Sprintf("got location: %v", g))
		mts.Meta.Add(mts.LocationKey, g)
	}

	return nil
}

// fileSender implements loadSender for a local file destination.
type fileSender struct {
	file        *os.File
	data        []byte
	multiFile   bool
	maxFileSize uint // maxFileSize is in bytes. A size of 0 means there is no size limit.
	path        string
	log         logging.Logger
}

// newFileSender returns a new fileSender. Setting multi true will write a new
// file for each write to this sender.
func newFileSender(l logging.Logger, path string, multiFile bool, maxFileSize uint) (*fileSender, error) {
	return &fileSender{
		path:        path,
		log:         l,
		multiFile:   multiFile,
		maxFileSize: maxFileSize,
	}, nil
}

// Write implements io.Writer.
func (s *fileSender) Write(d []byte) (int, error) {
	s.log.Debug("checking disk space")
	var stat syscall.Statfs_t
	if err := syscall.Statfs("/", &stat); err != nil {
		return 0, fmt.Errorf("could not read system disk space, abandoning write: %w", err)
	}
	availableSpace := stat.Bavail * uint64(stat.Bsize)
	totalSpace := stat.Blocks * uint64(stat.Bsize)
	s.log.Debug("available, total disk space in bytes", "availableSpace", availableSpace, "totalSpace", totalSpace)
	var spaceBuffer uint64 = 50000000 // 50MB.
	if availableSpace < spaceBuffer {
		return 0, fmt.Errorf("reached limit of disk space with a buffer of %v bytes, abandoning write", spaceBuffer)
	}

	// If the write will exceed the max file size, close the file so that a new one can be created.
	if s.maxFileSize != 0 && s.file != nil {
		fileInfo, err := s.file.Stat()
		if err != nil {
			return 0, fmt.Errorf("could not read files stats: %w", err)
		}
		size := uint(fileInfo.Size())
		s.log.Debug("checked file size", "bytes", size)
		if size+uint(len(d)) > s.maxFileSize {
			s.log.Debug("new write would exceed max file size, closing existing file", "maxFileSize", s.maxFileSize)
			s.file.Close()
			s.file = nil
		}
	}

	if s.file == nil {
		fileName := s.path + time.Now().Format("2006-01-02_15-04-05")
		s.log.Debug("creating new output file", "multiFile", s.multiFile, "fileName", fileName)
		f, err := os.Create(fileName)
		if err != nil {
			return 0, fmt.Errorf("could not create file to write media to: %w", err)
		}
		s.file = f
	}

	s.log.Debug("writing to output file", "bytes", len(d))
	n, err := s.file.Write(d)
	if err != nil {
		return n, err
	}

	if s.multiFile {
		s.file.Close()
		s.file = nil
	}

	return n, nil
}

func (s *fileSender) Close() error { return s.file.Close() }

// mtsSender implements io.WriteCloser and provides sending capability specifically
// for use with MPEGTS packetization. It handles the construction of appropriately
// lengthed clips based on clip duration and PSI. It also accounts for
// discontinuities by setting the discontinuity indicator for the first packet of a clip.
type mtsSender struct {
	dst      io.WriteCloser
	buf      []byte
	pool     *pool.Buffer
	next     []byte
	pkt      packet.Packet
	repairer *mts.DiscontinuityRepairer
	curPid   int
	clipDur  time.Duration
	prev     time.Time
	done     chan struct{}
	log      logging.Logger
	wg       sync.WaitGroup
}

// newMtsSender returns a new mtsSender.
func newMTSSender(dst io.WriteCloser, log logging.Logger, rb *pool.Buffer, clipDur time.Duration) *mtsSender {
	log.Debug("setting up mtsSender", "clip duration", int(clipDur))
	s := &mtsSender{
		dst:      dst,
		repairer: mts.NewDiscontinuityRepairer(),
		log:      log,
		pool:     rb,
		done:     make(chan struct{}),
		clipDur:  clipDur,
	}
	// mtsSender will do particularly large writes to the pool buffer; let's
	// increase its max allowable allocation.
	pool.MaxAlloc(mtsBufferPoolMaxAlloc)
	s.wg.Add(1)
	go s.output()
	return s
}

// output starts an mtsSender's data handling routine.
func (s *mtsSender) output() {
	var chunk *pool.Chunk
	for {
		select {
		case <-s.done:
			s.log.Info("terminating sender output routine")
			defer s.wg.Done()
			return
		default:
			// If chunk is nil then we're ready to get another from the ringBuffer.
			if chunk == nil {
				var err error
				chunk, err = s.pool.Next(mtsPoolReadTimeout)
				switch err {
				case nil, io.EOF:
					continue
				case pool.ErrTimeout:
					s.log.Debug("mtsSender: pool buffer read timeout")
					continue
				default:
					s.log.Error("unexpected error", "error", err.Error())
					continue
				}
			}
			err := s.repairer.Repair(chunk.Bytes())
			if err != nil {
				chunk.Close()
				chunk = nil
				continue
			}
			s.log.Debug("mtsSender: writing")
			_, err = s.dst.Write(chunk.Bytes())
			if err != nil {
				s.log.Debug("failed write, repairing MTS", "error", err)
				s.repairer.Failed()
				continue
			} else {
				s.log.Debug("good write")
			}
			chunk.Close()
			chunk = nil
		}
	}
}

// Write implements io.Writer.
func (s *mtsSender) Write(d []byte) (int, error) {
	if len(d) < mts.PacketSize {
		return 0, errors.New("do not have full MTS packet")
	}

	if s.next != nil {
		s.log.Debug("appending packet to clip")
		s.buf = append(s.buf, s.next...)
	}
	bytes := make([]byte, len(d))
	copy(bytes, d)
	s.next = bytes
	p, _ := mts.PID(bytes)
	s.curPid = int(p)
	curDur := time.Now().Sub(s.prev)
	s.log.Debug("checking send conditions", "curDuration", int(curDur), "sendDur", int(s.clipDur), "curPID", s.curPid, "len", len(s.buf))
	if curDur >= s.clipDur && s.curPid == mts.PatPid && len(s.buf) > 0 {
		s.log.Debug("writing clip to pool buffer for sending", "size", len(s.buf))
		s.prev = time.Now()
		n, err := s.pool.Write(s.buf)
		if err == nil {
			s.pool.Flush()
		}
		if err != nil {
			s.log.Warning("ringBuffer write error", "error", err.Error(), "n", n, "writeSize", len(s.buf), "rbElementSize", adjustedMTSPoolElementSize)
			if err == pool.ErrTooLong {
				adjustedMTSPoolElementSize = len(s.buf) * 2
				numElements := maxBuffLen / adjustedMTSPoolElementSize
				s.pool = pool.NewBuffer(maxBuffLen/adjustedMTSPoolElementSize, adjustedMTSPoolElementSize, 5*time.Second)
				s.log.Info("adjusted MTS pool buffer element size", "new size", adjustedMTSPoolElementSize, "num elements", numElements, "size(MB)", numElements*adjustedMTSPoolElementSize)
			}
		}
		s.buf = s.buf[:0]
	}
	return len(d), nil
}

// Close implements io.Closer.
func (s *mtsSender) Close() error {
	s.log.Debug("closing sender output routine")
	close(s.done)
	s.wg.Wait()
	s.log.Info("sender output routine closed")
	return nil
}

// rtmpSender implements loadSender for a native RTMP destination.
type rtmpSender struct {
	conn    *rtmp.Conn
	url     string
	retries int
	log     logging.Logger
	pool    *pool.Buffer
	done    chan struct{}
	wg      sync.WaitGroup
	report  func(sent int)
}

func newRtmpSender(url string, retries int, rb *pool.Buffer, log logging.Logger, report func(sent int)) (*rtmpSender, error) {
	var conn *rtmp.Conn
	var err error
	for n := 0; n < retries; n++ {
		conn, err = rtmp.Dial(url, log.Log)
		if err == nil {
			break
		}
		log.Error("dial error", "error", err)
		if n < retries-1 {
			log.Info("retrying dial")
		}
	}
	s := &rtmpSender{
		conn:    conn,
		url:     url,
		retries: retries,
		log:     log,
		pool:    rb,
		done:    make(chan struct{}),
		report:  report,
	}
	s.wg.Add(1)
	go s.output()
	return s, err
}

// output starts an mtsSender's data handling routine.
func (s *rtmpSender) output() {
	var chunk *pool.Chunk
	for {
		select {
		case <-s.done:
			s.log.Info("terminating sender output routine")
			defer s.wg.Done()
			return
		default:
			// If chunk is nil then we're ready to get another from the pool buffer.
			if chunk == nil {
				var err error
				chunk, err = s.pool.Next(rtmpPoolReadTimeout)
				switch err {
				case nil, io.EOF:
					continue
				case pool.ErrTimeout:
					s.log.Debug("rtmpSender: pool buffer read timeout")
					continue
				default:
					s.log.Error("unexpected error", "error", err.Error())
					continue
				}
			}
			if s.conn == nil {
				s.log.Warning("no rtmp connection, re-dialing")
				err := s.restart()
				if err != nil {
					s.log.Warning("could not restart connection", "error", err)
					continue
				}
			}
			_, err := s.conn.Write(chunk.Bytes())
			switch err {
			case nil, rtmp.ErrInvalidFlvTag:
				s.log.Debug("good write to conn")
			default:
				s.log.Warning("send error, re-dialing", "error", err)
				err = s.restart()
				if err != nil {
					s.log.Warning("could not restart connection", "error", err)
				}
				continue
			}
			chunk.Close()
			chunk = nil
		}
	}
}

// Write implements io.Writer.
func (s *rtmpSender) Write(d []byte) (int, error) {
	s.log.Debug("writing to pool buffer")
	_, err := s.pool.Write(d)
	if err == nil {
		s.pool.Flush()
		s.log.Debug("good pool buffer write", "len", len(d))
	} else {
		s.log.Warning("pool buffer write error", "error", err.Error())
		if err == pool.ErrTooLong {
			adjustedRTMPPoolElementSize = len(d) * 2
			numElements := maxBuffLen / adjustedRTMPPoolElementSize
			s.pool = pool.NewBuffer(numElements, adjustedRTMPPoolElementSize, 5*time.Second)
			s.log.Info("adjusted RTMP pool buffer element size", "new size", adjustedRTMPPoolElementSize, "num elements", numElements, "size(MB)", numElements*adjustedRTMPPoolElementSize)
		}
	}
	s.report(len(d))
	return len(d), nil
}

func (s *rtmpSender) restart() error {
	s.close()
	var err error
	for n := 0; n < s.retries; n++ {
		s.log.Debug("dialing", "dials", n)
		s.conn, err = rtmp.Dial(s.url, s.log.Log)
		if err == nil {
			break
		}
		s.log.Error("dial error", "error", err)
		if n < s.retries-1 {
			s.log.Info("retry rtmp connection")
		}
	}
	return err
}

func (s *rtmpSender) Close() error {
	s.log.Debug("closing output routine")
	if s.done != nil {
		close(s.done)
	}
	s.wg.Wait()
	s.log.Info("output routine closed")
	return s.close()
}

func (s *rtmpSender) close() error {
	s.log.Debug("closing connection")
	if s.conn == nil {
		return nil
	}
	return s.conn.Close()
}

// TODO: Write restart func for rtpSender
// rtpSender implements loadSender for a native udp destination with rtp packetization.
type rtpSender struct {
	log     logging.Logger
	encoder *rtp.Encoder
	data    []byte
	report  func(sent int)
}

func newRtpSender(addr string, log logging.Logger, fps uint, report func(sent int)) (*rtpSender, error) {
	conn, err := net.Dial("udp", addr)
	if err != nil {
		return nil, err
	}
	s := &rtpSender{
		log:     log,
		encoder: rtp.NewEncoder(conn, int(fps)),
		report:  report,
	}
	return s, nil
}

// Write implements io.Writer.
func (s *rtpSender) Write(d []byte) (int, error) {
	s.data = make([]byte, len(d))
	copy(s.data, d)
	_, err := s.encoder.Write(s.data)
	if err != nil {
		s.log.Warning("rtpSender: write error", err.Error())
	}
	s.report(len(d))
	return len(d), nil
}

func (s *rtpSender) Close() error { return nil }
