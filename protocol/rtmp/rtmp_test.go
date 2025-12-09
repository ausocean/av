/*
NAME
  rtmp_test.go

DESCRIPTION
  RTMP tests

AUTHORS
  Saxon Nelson-Milton <saxon@ausocean.org>
  Dan Kortschak <dan@ausocean.org>
  Alan Noble <alan@ausocean.org>

LICENSE
  Copyright (C) 2024 the Australian Ocean Lab (AusOcean). All Rights Reserved.

  The Software and all intellectual property rights associated
  therewith, including but not limited to copyrights, trademarks,
  patents, and trade secrets, are and will remain the exclusive
  property of the Australian Ocean Lab (AusOcean).
*/

package rtmp

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"runtime"
	"testing"
	"time"

	"github.com/ausocean/av/codec/h264"
	"github.com/ausocean/av/container/flv"
)

const (
	rtmpProtocol = "rtmp"
	testHost     = "a.rtmp.youtube.com"
	testApp      = "live2"
	testBaseURL  = rtmpProtocol + "://" + testHost + "/" + testApp + "/"
	testTimeout  = 30
	testDataDir  = "../../test/av/input"
)

// testVerbosity controls the amount of output.
// NB: This is not the log level, which is DebugLevel.
//
//	0: suppress logging completely
//	1: log messages only
//	2: log messages with errors, if any
var testVerbosity = 1

// testKey is the YouTube RTMP key required for YouTube streaming (RTMP_TEST_KEY env var).
// NB: don't share your key with others.
var testKey string

// testFile is the test video file  (RTMP_TEST_FILE env var).
// betterInput.h264 is a good one to use.
var testFile string

// testLog is a bare bones logger that logs to stdout, and exits upon either an error or fatal error.
func testLog(level int8, msg string, params ...interface{}) {
	logLevels := [...]string{"Debug", "Info", "Warn", "Error", "", "", "Fatal"}
	if testVerbosity == 0 {
		return
	}
	if level < -1 || level > 5 {
		panic("Invalid log level")
	}
	switch testVerbosity {
	case 0:
		// silence is golden
	case 1:
		fmt.Printf("%s: %s\n", logLevels[level+1], msg)
	case 2:
		// extract the first param if it is one we care about, otherwise just print the message
		if len(params) >= 2 {
			switch params[0].(string) {
			case "error":
				fmt.Printf("%s: %s, error=%v\n", logLevels[level+1], msg, params[1].(string))
			case "size":
				fmt.Printf("%s: %s, size=%d\n", logLevels[level+1], msg, params[1].(int))
			default:
				fmt.Printf("%s: %s\n", logLevels[level+1], msg)
			}
		} else {
			fmt.Printf("%s: %s\n", logLevels[level+1], msg)
		}
	}
	if level >= 4 {
		// Error or Fatal
		buf := make([]byte, 1<<16)
		size := runtime.Stack(buf, true)
		fmt.Printf("%s\n", string(buf[:size]))
		os.Exit(1)
	}
}

// TestKey tests that the RTMP_TEST_KEY environment variable is present
func TestKey(t *testing.T) {
	testLog(0, "TestKey")
	testKey = os.Getenv("RTMP_TEST_KEY")
	if testKey == "" {
		msg := "RTMP_TEST_KEY environment variable not defined"
		testLog(0, msg)
		t.Skip(msg)
	}
	testLog(0, "Testing against URL "+testBaseURL+testKey)
}

// TestErrorHandling tests error handling
func TestErorHandling(t *testing.T) {
	testLog(0, "TestErrorHandling")
	if testKey == "" {
		t.Skip("Skipping TestErrorHandling since no RTMP_TEST_KEY")
	}
	c, err := Dial(testBaseURL+testKey, testLog, LinkTimeout(testTimeout))
	if err != nil {
		t.Errorf("Dial failed with error: %v", err)
	}

	// test the link parts are as expected
	if c.link.protocol&featureWrite == 0 {
		t.Errorf("link not writable")
	}
	if rtmpProtocolStrings[c.link.protocol&^featureWrite] != rtmpProtocol {
		t.Errorf("wrong protocol: %v", c.link.protocol)
	}
	if c.link.host != testHost {
		t.Errorf("wrong host: %v", c.link.host)
	}
	if c.link.app != testApp {
		t.Errorf("wrong app: %v", c.link.app)
	}

	// test errInvalidFlvTag
	var buf [1024]byte
	tag := buf[:0]
	_, err = c.Write(tag)
	if err == nil {
		t.Errorf("Write did not return errInvalidFlvTag")
	}

	// test errUnimplemented
	copy(tag, []byte("FLV"))
	_, err = c.Write(tag)
	if err == nil {
		t.Errorf("Write did not return errUnimplemented")
	}

	// test errInvalidBody
	tag = buf[:11]
	_, err = c.Write(tag)
	if err == nil {
		t.Errorf("Write did not return errInvalidBody")
	}

	err = c.Close()
	if err != nil {
		t.Errorf("Close failed with error: %v", err)
		return
	}
}

// TestFromFrame tests streaming from a single H.264 frame which is repeated.
func TestFromFrame(t *testing.T) {
	testLog(0, "TestFromFrame")
	testFrame := os.Getenv("RTMP_TEST_FRAME")
	if testFrame == "" {
		t.Skip("Skipping TestFromFrame since no RTMP_TEST_FRAME")
	}
	if testKey == "" {
		t.Skip("Skipping TestFromFrame since no RTMP_TEST_KEY")
	}
	c, err := Dial(testBaseURL+testKey, testLog, LinkTimeout(testTimeout))
	if err != nil {
		t.Errorf("Dial failed with error: %v", err)
	}

	b, err := ioutil.ReadFile(testFrame)
	if err != nil {
		t.Errorf("ReadFile failed with error: %v", err)
	}

	const noOfFrames = 1000
	videoData := make([]byte, 0, noOfFrames*len(b))
	for i := 0; i < noOfFrames; i++ {
		videoData = append(videoData, b...)
	}

	const frameRate = 25
	rs := &rtmpSender{conn: c}
	flvEncoder, err := flv.NewEncoder(rs, true, frameRate)
	if err != nil {
		t.Errorf("Failed to create flv encoder with error: %v", err)
	}
	videoAdapter := &flv.DummyAudioDecorator{Encoder: flvEncoder}
	err = h264.Lex(videoAdapter, bytes.NewReader(videoData), time.Second/time.Duration(frameRate))
	if err != nil {
		t.Errorf("Lexing failed with error: %v", err)
	}

	err = c.Close()
	if err != nil {
		t.Errorf("Conn.Close failed with error: %v", err)
	}
}

type rtmpSender struct {
	conn *Conn
}

func (rs *rtmpSender) Write(p []byte) (int, error) {
	n, err := rs.conn.Write(p)
	if err != ErrInvalidFlvTag && err != nil {
		return 0, err
	}
	return n, nil
}

func (rs *rtmpSender) Close() error { return nil }

// TestFromFile tests streaming from an video file comprising raw H.264.
// The test file is supplied via the RTMP_TEST_FILE environment variable.
func TestFromFile(t *testing.T) {
	testLog(0, "TestFromFile")
	testFile := os.Getenv("RTMP_TEST_FILE")
	if testFile == "" {
		t.Skip("Skipping TestFromFile since no RTMP_TEST_FILE")
	}
	if testKey == "" {
		t.Skip("Skipping TestFromFile since no RTMP_TEST_KEY")
	}
	c, err := Dial(testBaseURL+testKey, testLog, LinkTimeout(testTimeout))
	if err != nil {
		t.Errorf("Dial failed with error: %v", err)
	}
	f, err := os.Open(testFile)
	if err != nil {
		t.Errorf("Open failed with error: %v", err)
	}
	defer f.Close()

	rs := &rtmpSender{conn: c}
	// Pass RTMP session, true for stereo, and 25 FPS.
	flvEncoder, err := flv.NewEncoder(rs, true, 25)
	if err != nil {
		t.Fatalf("failed to create encoder: %v", err)
	}
	videoAdapter := &flv.DummyAudioDecorator{Encoder: flvEncoder}
	err = h264.Lex(videoAdapter, f, time.Second/time.Duration(25))
	if err != nil {
		t.Errorf("Lexing and encoding failed with error: %v", err)
	}

	err = c.Close()
	if err != nil {
		t.Errorf("Conn.Close failed with error: %v", err)
	}
}
