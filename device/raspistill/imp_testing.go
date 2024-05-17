// +build test

/*
DESCRIPTION
  test.go provides test implementations of the raspistill methods when the
  "test" build tag is specified. In this mode, raspistill simply provides
  specific test JPEG images when Raspistill.Read() is called.

AUTHORS
  Saxon Nelson-Milton <saxon@ausocean.org>

LICENSE
  Copyright (C) 2024 the Australian Ocean Lab (AusOcean). All Rights Reserved. 

  The Software and all intellectual property rights associated
  therewith, including but not limited to copyrights, trademarks,
  patents, and trade secrets, are and will remain the exclusive
  property of the Australian Ocean Lab (AusOcean).
*/


package raspistill

import (
	"io"
	"io/ioutil"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/ausocean/av/revid/config"
)

const (
	// TODO(Saxon): find nImages programmatically ?
	nImages       = 6
	relImgPath    = "/github.com/ausocean/test/test-data/av/input/jpeg/"
	jpgExt        = ".jpg"
	gopathEnvName = "GOPATH"
	readWaitDelay = 1 * time.Second
)

type raspistill struct {
	images      [nImages][]byte
	imgCnt      int          // Number of images that have been loaded thus far.
	durTicker   *time.Ticker // Tracks timelapse duration.
	intvlTicker *time.Ticker // Tracks current interval in the timelapse.
	log         logging.Logger
	cfg         config.Config
	isRunning   bool
	buf         []byte        // Holds frame data to be read.
	term        chan struct{} // Signals termination when close() is called.
	mu          sync.Mutex
}

func new(l logging.Logger) raspistill {
	l.Debug("creating new test raspistill input")

	r := raspistill{log: l}

	// Load the test images into the images slice.
	// We expect the 6 images test images to be named 0.jpg through to 5.jpg.
	r.log.Debug("loading test JPEG images")
	for i, _ := range r.images {
		imgDir := os.Getenv(gopathEnvName) + relImgPath
		path := imgDir + strconv.Itoa(i) + jpgExt

		var err error
		r.images[i], err = ioutil.ReadFile(path)
		if err != nil {
			r.log.Fatal("error loading test image", "imageNum", i, "error", err)
		}
		r.log.Debug("image loaded", "path/name", path, "size", len(r.images[i]))
	}
	return r
}

// stop sets isRunning flag to false, indicating no further captures. Future
// calls on Raspistill.read will return an error.
func (r *Raspistill) stop() error {
	r.log.Debug("stopping test raspistill")
	if r.running() {
		r.setRunning(false)
		close(r.term)
	}
	return nil
}

// start creates and starts the timelapse and duration tickers and sets
// isRunning flag to true indicating that raspistill is capturing.
func (r *Raspistill) start() error {
	r.log.Debug("starting test implementation raspistill", "duration", r.cfg.TimelapseDuration.String(), "interval", r.cfg.TimelapseInterval.String())
	r.durTicker = time.NewTicker(r.cfg.TimelapseDuration)
	r.intvlTicker = time.NewTicker(r.cfg.TimelapseInterval)

	r.term = make(chan struct{})

	r.loadImg()

	go r.capture()

	r.setRunning(true)

	return nil
}

func (r *Raspistill) loadImg() {
	r.log.Debug("appending new image on to buffer and copying next image p", "nImg", r.imgCnt)
	imgBytes := r.images[r.imgCnt%nImages]
	if len(imgBytes) == 0 {
		panic("length of image bytes should not be 0")
	}
	r.imgCnt++

	r.mu.Lock()
	r.buf = append(r.buf, imgBytes...)
	r.log.Debug("added image to buf", "nBytes", len(imgBytes))
	r.mu.Unlock()
}

func (r *Raspistill) capture() {
	for {
		select {
		case t := <-r.intvlTicker.C:
			r.log.Debug("got interval tick", "tick", t)
			r.loadImg()
			r.intvlTicker.Reset(r.cfg.TimelapseInterval)

		case <-r.term:
			r.log.Debug("got termination signal")
			r.setRunning(false)
			return

		case t := <-r.durTicker.C:
			r.log.Debug("got duration tick, timelapse over", "tick", t)
			close(r.term)
		}
	}
}

// read blocks until either another timelapse interval has completed, in which
// case we provide the next jpeg to p, or, the timelapse duration has completed
// in which case we don't read and provide io.EOF error.
func (r *Raspistill) read(p []byte) (int, error) {
	r.log.Debug("reading from test raspistill")
	if !r.running() {
		return 0, io.EOF
	}

	// Waits until there's something in the buffer for reading, unless there's a
	// termination signal, in which case return io.EOF.
	for {
		select {
		case <-r.term:
			return 0, io.EOF
		default:
		}

		if r.bufLen() != 0 {
			break
		}
		time.Sleep(readWaitDelay)
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	n := copy(p, r.buf)
	r.buf = r.buf[n:]

	return n, nil
}

func (r *Raspistill) bufLen() int {
	r.mu.Lock()
	l := len(r.buf)
	r.mu.Unlock()
	return l
}

func (r *Raspistill) setRunning(s bool) {
	r.mu.Lock()
	r.isRunning = s
	r.mu.Unlock()
}

func (r *Raspistill) running() bool {
	r.mu.Lock()
	ir := r.isRunning
	r.mu.Unlock()
	return ir
}
