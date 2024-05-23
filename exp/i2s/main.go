package main

import (
	"log"
	"os"
	"time"

	"github.com/ausocean/av/codec/codecutil"
	"github.com/ausocean/av/device/alsa"
	"github.com/ausocean/av/revid/config"
	"github.com/ausocean/utils/logging"
)

func main() {
	// Setup a logger.
	logfile, err := os.Create("i2s.log")
	if err != nil {
		log.Println("error making logfile:", err)
		return
	}
	l := logging.New(logging.Debug, logfile, false)

	// Setup an ALSA device.
	dev := alsa.New(l)

	cfg := config.Config{
		SampleRate: 48000,
		Channels: 1,
		BitDepth: 16,
		RecPeriod: 1,
		InputCodec: codecutil.PCM,
	}

	err = dev.Setup(cfg)
	if err != nil {
		l.Fatal("unable to setup device", "err",  err)
		return
	}

	err = dev.Start()
	if err != nil {
		l.Fatal("could not start device", "err", err)
		return
	}

	l.Debug("number of bytes per recPeriod", "Datasize", dev.DataSize())

	// Open a file to write to.
	output, err := os.Create("output.pcm")
	if err != nil {
		l.Fatal("unable to create file", "err",  err)
		return
	}

	lexer, err := codecutil.NewByteLexer(dev.DataSize())
	if err != nil {
		l.Fatal("could not create bytelexer", "err",  err)
		return
	}

	err = lexer.Lex(output, dev, time.Duration(dev.RecPeriod*float64(time.Second)))
	// We should never get here...
	if err != nil {
		l.Fatal("failed lexing", "err", err)
		return
	}

}