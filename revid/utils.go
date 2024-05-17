package revid

import (
	"testing"

	"github.com/ausocean/utils/logging"
)

// testLogger will allow logging to be done by the testing pkg.
type testLogger testing.T

func (tl *testLogger) Debug(msg string, args ...interface{})   { tl.Log(logging.Debug, msg, args...) }
func (tl *testLogger) Info(msg string, args ...interface{})    { tl.Log(logging.Info, msg, args...) }
func (tl *testLogger) Warning(msg string, args ...interface{}) { tl.Log(logging.Warning, msg, args...) }
func (tl *testLogger) Error(msg string, args ...interface{})   { tl.Log(logging.Error, msg, args...) }
func (tl *testLogger) Fatal(msg string, args ...interface{})   { tl.Log(logging.Fatal, msg, args...) }
func (tl *testLogger) SetLevel(lvl int8)                       {}
func (dl *testLogger) Log(lvl int8, msg string, args ...interface{}) {
	var l string
	switch lvl {
	case logging.Warning:
		l = "warning"
	case logging.Debug:
		l = "debug"
	case logging.Info:
		l = "info"
	case logging.Error:
		l = "error"
	case logging.Fatal:
		l = "fatal"
	}
	msg = l + ": " + msg

	// Just use test.T.Log if no formatting required.
	if len(args) == 0 {
		((*testing.T)(dl)).Log(msg)
		return
	}

	// Add braces with args inside to message.
	msg += " ("
	for i := 0; i < len(args); i += 2 {
		msg += " %v:\"%v\""
	}
	msg += " )"

	if lvl == logging.Fatal {
		dl.Fatalf(msg+"\n", args...)
	}

	dl.Logf(msg+"\n", args...)
}
