// +build test

/*
DESCRIPTION
  revid_test.go provides integration testing of the revid API.

AUTHORS
  Saxon A. Nelson-Milton <saxon@ausocean.org>

LICENSE
  Copyright (C) 2024 the Australian Ocean Lab (AusOcean). All Rights Reserved. 

  The Software and all intellectual property rights associated
  therewith, including but not limited to copyrights, trademarks,
  patents, and trade secrets, are and will remain the exclusive
  property of the Australian Ocean Lab (AusOcean).
*/


package revid

import (
	"bytes"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/ausocean/av/container/mts"
	"github.com/ausocean/av/container/mts/meta"
	"github.com/ausocean/av/revid/config"
)

func TestRaspistill(t *testing.T) {
	// Copyright information prefixed to all metadata.
	const (
		metaPreambleKey  = "copyright"
		metaPreambleData = "ausocean.org/license/content2021"
	)

	// Configuration parameters.
	const (
		timelapseInterval    = "4"
		timelapseDuration    = "25"
		poolStartElementSize = "1000000"
		input                = "Raspistill"
		codec                = "JPEG"
		output               = "Files"
		outDir               = "out"
		outputPath           = outDir + "/"
		logging              = "Debug"
		testImgDir           = "../../../test/test-data/av/input/jpeg/"
	)

	const runTime = 40 * time.Second

	// Add the copyright metadata.
	mts.Meta = meta.NewWith([][2]string{{metaPreambleKey, metaPreambleData}})

	// First remove out dir (if exists) to clear contents, then create dir.
	err := os.RemoveAll(outDir)
	if err != nil {
		t.Fatalf("could not remove any prior out directory: %v", err)
	}

	os.Mkdir(outDir, os.ModePerm)
	if err != nil {
		t.Fatalf("could not create new out directory: %v", err)
	}

	rv, err := New(config.Config{Logger: (*testLogger)(t)}, nil)
	if err != nil {
		t.Fatalf("did not expect error from revid.New(): %v", err)
	}

	err = rv.Update(
		map[string]string{
			config.KeyInput:                input,
			config.KeyInputCodec:           codec,
			config.KeyOutput:               output,
			config.KeyOutputPath:           outputPath,
			config.KeyTimelapseInterval:    timelapseInterval,
			config.KeyTimelapseDuration:    timelapseDuration,
			config.KeyLogging:              logging,
			config.KeyPoolStartElementSize: poolStartElementSize,
		},
	)
	if err != nil {
		t.Fatalf("did not expect error from rv.Update(): %v", err)
	}

	err = rv.Start()
	if err != nil {
		t.Fatalf("did not expect error from rv.Start(): %v", err)
	}
	time.Sleep(runTime)
	rv.Stop()

	// Get output file information.
	os.Chdir(outDir)
	var files []string
	err = filepath.Walk(
		".",
		func(path string, info os.FileInfo, err error) error {
			files = append(files, path)
			return nil
		},
	)
	if err != nil {
		t.Fatalf("did not expect error from filepath.Walk(): %v", err)
	}

	if len(files) == 0 {
		t.Fatalf("did not expect 0 output files")
	}

	// Load files outputted files and compare each one with corresponding input.
	for i, n := range files {
		// Ignore first file (which is prev dir ".").
		if i == 0 {
			continue
		}

		mtsBytes, err := ioutil.ReadFile(n)
		if err != nil {
			t.Fatalf("could not read output file: %v", err)
		}

		clip, err := mts.Extract(mtsBytes)
		if err != nil {
			t.Fatalf("could not extract clips from MPEG-TS stream: %v", err)
		}
		img := clip.Bytes()

		inImg, err := ioutil.ReadFile(testImgDir + strconv.Itoa(i) + ".jpg")
		if err != nil {
			t.Fatalf("could not load input test image: %v", err)
		}

		if !bytes.Equal(img, inImg) {
			t.Errorf("unexpected image extracted for img: %d", i)
		}
	}

	// Clean up out directory.
	err = os.RemoveAll(outDir)
	if err != nil {
		t.Fatalf("could not clean up out directory: %v", err)
	}
}
