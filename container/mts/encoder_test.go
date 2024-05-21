/*
NAME
  encoder_test.go

AUTHOR
  Saxon A. Nelson-Milton <saxon@ausocean.org>
  Trek Hopton <trek@ausocean.org>

LICENSE
  Copyright (C) 2024 the Australian Ocean Lab (AusOcean). All Rights Reserved.

  The Software and all intellectual property rights associated
  therewith, including but not limited to copyrights, trademarks,
  patents, and trade secrets, are and will remain the exclusive
  property of the Australian Ocean Lab (AusOcean).
*/

package mts

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"reflect"
	"testing"

	"github.com/Comcast/gots/v2/packet"
	"github.com/Comcast/gots/v2/pes"

	"github.com/ausocean/av/container/mts/meta"
	"github.com/ausocean/av/container/mts/psi"
	"github.com/ausocean/utils/logging"
)

type nopCloser struct{ io.Writer }

func (nopCloser) Close() error { return nil }

type destination struct {
	packets [][]byte
}

func (d *destination) Write(p []byte) (int, error) {
	tmp := make([]byte, PacketSize)
	copy(tmp, p)
	d.packets = append(d.packets, tmp)
	return len(p), nil
}

// TestEncodeVideo checks that we can correctly encode some dummy data into a
// valid MPEG-TS stream. This checks for correct MPEG-TS headers and also that the
// original data is stored correctly and is retreivable.
func TestEncodeVideo(t *testing.T) {
	Meta = meta.New()

	const dataLength = 440
	const numOfPackets = 3
	const stuffingLen = 100

	// Generate test data.
	data := make([]byte, 0, dataLength)
	for i := 0; i < dataLength; i++ {
		data = append(data, byte(i))
	}

	// Expect headers for PID 256 (video)
	// NB: timing fields like PCR are neglected.
	expectedHeaders := [][]byte{
		{
			0x47, // Sync byte.
			0x41, // TEI=0, PUSI=1, TP=0, PID=00001 (256).
			0x00, // PID(Cont)=00000000.
			0x30, // TSC=00, AFC=11(adaptation followed by payload), CC=0000(0).
			0x07, // AFL= 7.
			0x50, // DI=0,RAI=1,ESPI=0,PCRF=1,OPCRF=0,SPF=0,TPDF=0, AFEF=0.
		},
		{
			0x47, // Sync byte.
			0x01, // TEI=0, PUSI=0, TP=0, PID=00001 (256).
			0x00, // PID(Cont)=00000000.
			0x31, // TSC=00, AFC=11(adaptation followed by payload), CC=0001(1).
			0x01, // AFL= 1.
			0x00, // DI=0,RAI=0,ESPI=0,PCRF=0,OPCRF=0,SPF=0,TPDF=0, AFEF=0.
		},
		{
			0x47, // Sync byte.
			0x01, // TEI=0, PUSI=0, TP=0, PID=00001 (256).
			0x00, // PID(Cont)=00000000.
			0x32, // TSC=00, AFC=11(adaptation followed by payload), CC=0010(2).
			0x57, // AFL= 1+stuffingLen.
			0x00, // DI=0,RAI=0,ESPI=0,PCRF=1,OPCRF=0,SPF=0,TPDF=0, AFEF=0.
		},
	}

	// Create the dst and write the test data to encoder.
	dst := &destination{}
	e, err := NewEncoder(nopCloser{dst}, (*logging.TestLogger)(t), PacketBasedPSI(psiSendCount), Rate(25), MediaType(EncodeH264))
	if err != nil {
		t.Fatalf("could not create MTS encoder, failed with error: %v", err)
	}

	_, err = e.Write(data)
	if err != nil {
		t.Fatalf("could not write data to encoder, failed with error: %v\n", err)
	}

	// Check headers.
	var expectedIdx int
	for _, p := range dst.packets {
		// Get PID.
		var _p packet.Packet
		copy(_p[:], p)
		pid := packet.Pid(&_p)
		if pid == PIDVideo {
			// Get mts header, excluding PCR.
			gotHeader := p[0:6]
			wantHeader := expectedHeaders[expectedIdx]
			if !bytes.Equal(gotHeader, wantHeader) {
				t.Errorf("did not get expected header for idx: %v.\n Got: %v\n Want: %v\n", expectedIdx, gotHeader, wantHeader)
			}
			expectedIdx++
		}
	}

	// Gather payload data from packets to form the total PES packet.
	var pesData []byte
	for _, p := range dst.packets {
		var _p packet.Packet
		copy(_p[:], p)
		pid := packet.Pid(&_p)
		if pid == PIDVideo {
			payload, err := packet.Payload(&_p)
			if err != nil {
				t.Fatalf("could not get payload from mts packet, failed with err: %v\n", err)
			}
			pesData = append(pesData, payload...)
		}
	}

	// Get data from the PES packet and compare with the original data.
	pes, err := pes.NewPESHeader(pesData)
	if err != nil {
		t.Fatalf("got error from pes creation: %v\n", err)
	}
	_data := pes.Data()
	if !bytes.Equal(data, _data) {
		t.Errorf("did not get expected result.\n Got: %v\n Want: %v\n", data, _data)
	}
}

// TestEncodePcm tests the MPEG-TS encoder's ability to encode pcm audio data.
// It reads and encodes input pcm data into MPEG-TS, then decodes the MPEG-TS and compares the result to the input pcm.
func TestEncodePcm(t *testing.T) {
	Meta = meta.New()

	var buf bytes.Buffer
	sampleRate := 48000
	sampleSize := 2
	blockSize := 16000
	writeFreq := float64(sampleRate*sampleSize) / float64(blockSize)
	e, err := NewEncoder(nopCloser{&buf}, (*logging.TestLogger)(t), PacketBasedPSI(10), Rate(writeFreq), MediaType(EncodePCM))
	if err != nil {
		t.Fatalf("could not create MTS encoder, failed with error: %v", err)
	}

	inPath := "../../../test/test-data/av/input/sweep_400Hz_20000Hz_-3dBFS_5s_48khz.pcm"
	inPcm, err := ioutil.ReadFile(inPath)
	if err != nil {
		t.Errorf("unable to read file: %v", err)
	}

	// Break pcm into blocks and encode to mts and get the resulting bytes.
	for i := 0; i < len(inPcm); i += blockSize {
		if len(inPcm)-i < blockSize {
			block := inPcm[i:]
			_, err = e.Write(block)
			if err != nil {
				t.Errorf("unable to write block: %v", err)
			}
		} else {
			block := inPcm[i : i+blockSize]
			_, err = e.Write(block)
			if err != nil {
				t.Errorf("unable to write block: %v", err)
			}
		}
	}
	clip := buf.Bytes()

	// Get the first MTS packet to check
	var pkt packet.Packet
	pesPacket := make([]byte, 0, blockSize)
	got := make([]byte, 0, len(inPcm))
	i := 0
	if i+PacketSize <= len(clip) {
		copy(pkt[:], clip[i:i+PacketSize])
	}

	// Loop through MTS packets until all the audio data from PES packets has been retrieved
	for i+PacketSize <= len(clip) {

		// Check MTS packet
		if pkt.PID() != PIDAudio {
			i += PacketSize
			if i+PacketSize <= len(clip) {
				copy(pkt[:], clip[i:i+PacketSize])
			}
			continue
		}
		if !pkt.PayloadUnitStartIndicator() {
			i += PacketSize
			if i+PacketSize <= len(clip) {
				copy(pkt[:], clip[i:i+PacketSize])
			}
		} else {
			// Copy the first MTS payload
			payload, err := pkt.Payload()
			if err != nil {
				t.Errorf("unable to get MTS payload: %v", err)
			}
			pesPacket = append(pesPacket, payload...)

			i += PacketSize
			if i+PacketSize <= len(clip) {
				copy(pkt[:], clip[i:i+PacketSize])
			}

			// Copy the rest of the MTS payloads that are part of the same PES packet
			for (!pkt.PayloadUnitStartIndicator()) && i+PacketSize <= len(clip) {
				payload, err = pkt.Payload()
				if err != nil {
					t.Errorf("unable to get MTS payload: %v", err)
				}
				pesPacket = append(pesPacket, payload...)

				i += PacketSize
				if i+PacketSize <= len(clip) {
					copy(pkt[:], clip[i:i+PacketSize])
				}
			}
		}
		// Get the audio data from the current PES packet
		pesHeader, err := pes.NewPESHeader(pesPacket)
		if err != nil {
			t.Errorf("unable to read PES packet: %v", err)
		}
		got = append(got, pesHeader.Data()...)
		pesPacket = pesPacket[:0]
	}

	// Compare data from MTS with original data.
	if !bytes.Equal(got, inPcm) {
		t.Error("data decoded from mts did not match input data")
	}
}

const fps = 25

// TestMetaEncode1 checks that we can externally add a single metadata entry to
// the mts global Meta meta.Data struct and then successfully have the mts encoder
// write this to psi.
func TestMetaEncode1(t *testing.T) {
	Meta = meta.New()
	var buf bytes.Buffer
	e, err := NewEncoder(nopCloser{&buf}, (*logging.TestLogger)(t))
	if err != nil {
		t.Fatalf("could not create encoder, failed with error: %v", err)
	}

	Meta.Add(TimestampKey, "12345678")
	if err := e.writePSI(); err != nil {
		t.Errorf("unexpected error: %v\n", err.Error())
	}
	out := buf.Bytes()
	got := out[PacketSize+4:]

	want := []byte{
		0x00, 0x02, 0xb0, 0x37, 0x00, 0x01, 0xc1, 0x00, 0x00, 0xe1, 0x00, 0xf0, 0x25,
		psi.MetadataTag, // Descriptor tag
		0x23,            // Length of bytes to follow
		0x00, 0x10, 0x00, 0x1f,
	}
	rate := WriteRateKey + "=" + fmt.Sprintf("%f", float64(defaultRate)) + "\t" + TimestampKey + "=12345678" // timestamp
	want = append(want, []byte(rate)...)                                                                     // writeRate
	want = append(want, []byte{0x1b, 0xe1, 0x00, 0xf0, 0x00}...)
	want = psi.AddCRC(want)
	want = psi.AddPadding(want)
	if !bytes.Equal(got, want) {
		t.Errorf("unexpected output. \n Got : %v\n, Want: %v\n", got, want)
	}
}

// TestMetaEncode2 checks that we can externally add two metadata entries to the
// Meta meta.Data global and then have the mts encoder successfully encode this
// into psi.
func TestMetaEncode2(t *testing.T) {
	Meta = meta.New()
	var buf bytes.Buffer
	e, err := NewEncoder(nopCloser{&buf}, (*logging.TestLogger)(t))
	if err != nil {
		t.Fatalf("could not create MTS encoder, failed with error: %v", err)
	}

	Meta.Add(TimestampKey, "12345678")
	Meta.Add(LocationKey, "1234,4321,1234")
	if err := e.writePSI(); err != nil {
		t.Errorf("did not expect error: %v from writePSI", err.Error())
	}
	out := buf.Bytes()
	got := out[PacketSize+4:]
	want := []byte{
		0x00, 0x02, 0xb0, 0x4a, 0x00, 0x01, 0xc1, 0x00, 0x00, 0xe1, 0x00, 0xf0, 0x38,
		psi.MetadataTag, // Descriptor tag
		0x36,            // Length of bytes to follow
		0x00, 0x10, 0x00, 0x32,
	}
	s := WriteRateKey + "=" + fmt.Sprintf("%f", float64(defaultRate)) + "\t" +
		TimestampKey + "=12345678\t" + LocationKey + "=1234,4321,1234"
	want = append(want, []byte(s)...)
	want = append(want, []byte{0x1b, 0xe1, 0x00, 0xf0, 0x00}...)
	want = psi.AddCRC(want)
	want = psi.AddPadding(want)
	if !bytes.Equal(got, want) {
		t.Errorf("did not get expected results.\ngot: %v\nwant: %v\n", got, want)
	}
}

// TestExtractMeta checks that ExtractMeta can correclty get a map of metadata
// from the first PMT in a clip of MPEG-TS.
func TestExtractMeta(t *testing.T) {
	Meta = meta.New()
	var buf bytes.Buffer
	e, err := NewEncoder(nopCloser{&buf}, (*logging.TestLogger)(t))
	if err != nil {
		t.Fatalf("could not create MTS encoder, failed with error: %v", err)
	}

	Meta.Add(TimestampKey, "12345678")
	Meta.Add(LocationKey, "1234,4321,1234")
	if err := e.writePSI(); err != nil {
		t.Errorf("did not expect error: %v", err.Error())
	}
	out := buf.Bytes()
	got, err := ExtractMeta(out)
	if err != nil {
		t.Errorf("did not expect error: %v", err.Error())
	}
	rate := fmt.Sprintf("%f", float64(defaultRate))
	want := map[string]string{
		TimestampKey: "12345678",
		LocationKey:  "1234,4321,1234",
		WriteRateKey: rate,
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("did not get expected result.\ngot: %v\nwant: %v\n", got, want)
	}
}
