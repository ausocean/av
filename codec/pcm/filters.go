/*
NAME
  filters.go

DESCRIPTION
  filter.go contains functions for filtering PCM audio.

AUTHOR
  David Sutton <davidsutton@ausocean.org>

LICENSE
  Copyright (C) 2024 the Australian Ocean Lab (AusOcean). All Rights Reserved. 

  The Software and all intellectual property rights associated
  therewith, including but not limited to copyrights, trademarks,
  patents, and trade secrets, are and will remain the exclusive
  property of the Australian Ocean Lab (AusOcean).
*/


package pcm

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"math"

	"github.com/mjibson/go-dsp/fft"
	"github.com/mjibson/go-dsp/window"
)

// AudioFilter is an interface which contains an Apply function.
// Apply is used to apply the filter to the given buffer of PCM data (b.Data).
type AudioFilter interface {
	Apply(b Buffer) ([]byte, error)
}

// SelectiveFrequencyFilter is a struct which contains all the filter specifications required for a
// lowpass, highpass, bandpass, or bandstop filter.
type SelectiveFrequencyFilter struct {
	coeffs     []float64
	cutoff     [2]float64
	sampleRate uint
	taps       int
	buffInfo   BufferFormat
}

// NewLowPass populates a LowPass struct with the specified data. The function also
// generates a lowpass filter based off the given specifications, and returns a pointer.
func NewLowPass(fc float64, info BufferFormat, length int) (*SelectiveFrequencyFilter, error) {
	return newLoHiFilter(fc, info, length, [2]float64{0, fc})
}

// NewHighPass populates a HighPass struct with the specified data. The function also
// generates a highpass filter based off the given specifications, and returns a pointer.
func NewHighPass(fc float64, info BufferFormat, length int) (*SelectiveFrequencyFilter, error) {
	return newLoHiFilter(fc, info, length, [2]float64{fc, 0})
}

// NewBandPass populates a BandPass struct with the specified data. The function also
// generates a bandpass filter based off the given specifications, and returns a pointer.
func NewBandPass(fc_lower, fc_upper float64, info BufferFormat, length int) (*SelectiveFrequencyFilter, error) {
	newFilter, lp, hp, err := newBandFilter([2]float64{fc_lower, fc_upper}, info, length)
	if err != nil {
		return nil, fmt.Errorf("could not create new band filter: %w", err)
	}

	// Convolve the filters to create a bandpass filter.
	newFilter.coeffs, err = fastConvolve(hp.coeffs, lp.coeffs)
	if err != nil {
		return nil, fmt.Errorf("could not compute fast convolution: %w", err)
	}

	// Return a pointer to the filter.
	return newFilter, nil
}

// NewBandStop populates a BandStop struct with the specified data. The function also
// generates a bandstop filter based off the given specifications, and returns a pointer.
func NewBandStop(fc_lower, fc_upper float64, info BufferFormat, length int) (*SelectiveFrequencyFilter, error) {
	newFilter, lp, hp, err := newBandFilter([2]float64{fc_upper, fc_lower}, info, length)
	if err != nil {
		return nil, fmt.Errorf("could not create new band filter: %w", err)
	}
	size := newFilter.taps + 1
	newFilter.coeffs = make([]float64, size)
	for i := range lp.coeffs {
		newFilter.coeffs[i] = lp.coeffs[i] + hp.coeffs[i]
	}

	// Return a pointer to the filter.
	return newFilter, nil
}

// Apply is the SelectiveFrequencyFilter implementation of the AudioFilter interface. This implementation
// takes the buffer data (b.Data), applies the highpass filter and returns a byte slice of filtered audio.
func (filter *SelectiveFrequencyFilter) Apply(b Buffer) ([]byte, error) {
	// Apply the lowpass filter to the given audio buffer.
	return convolveFromBytes(b.Data, filter.coeffs)
}

// Amplifier is a struct which contains the factor of amplification to be used in the application
// of the filter.
type Amplifier struct {
	factor float64
}

// NewAmplifier defines the factor of amplification for an amplifying filter.
func NewAmplifier(factor float64) *Amplifier {
	// Return populated Amplifier filter.
	// Uses the absolute value of the factor to ensure compatibility.
	return &Amplifier{factor: math.Abs(factor)}
}

// Apply implemented for an amplifier takes the buffer data (b.Data), applies
// the amplification and returns a byte slice of filtered audio.
func (amp *Amplifier) Apply(b Buffer) ([]byte, error) {
	inputAsFloat, err := bytesToFloats(b.Data)
	if err != nil {
		return nil, fmt.Errorf("failed to convert to floats: %w", err)
	}

	// Multiply every sample by the factor of amplification.
	floatOutput := make([]float64, len(inputAsFloat))
	for i := range inputAsFloat {
		floatOutput[i] = inputAsFloat[i] * amp.factor
		// Stop audio artifacting by clipping outputs.
		if floatOutput[i] > 1 {
			floatOutput[i] = 1
		} else if floatOutput[i] < -1 {
			floatOutput[i] = -1
		}
	}
	outBytes, err := floatsToBytes(floatOutput)
	if err != nil {
		return nil, fmt.Errorf("failed to convert to bytes: %w", err)
	}
	return outBytes, nil
}

// newLoHiFilter is a function which checks for the validity of the input parameters, and calls the newLoHiFilter function
// to return a pointer to either a lowpass or a highpass filter.
func newLoHiFilter(fc float64, info BufferFormat, length int, cutoff [2]float64) (*SelectiveFrequencyFilter, error) {
	// Ensure that all input values are valid.
	if fc <= 0 || fc >= float64(info.Rate)/2 {
		return nil, errors.New("cutoff frequency out of bounds")
	} else if length <= 0 {
		return nil, errors.New("cannot create filter with length <= 0")
	}

	// Determine the type of filter to be generated.
	var fd float64
	var factor1 float64
	var factor2 float64
	if cutoff[0] == 0 { // For a lowpass filter, cutoff[0] = 0, cutoff[1] = fc.
		// The filter must be a lowpass filter.
		fd = cutoff[1] / float64(info.Rate)
		factor1 = 1
		factor2 = 2 * fd
	} else if cutoff[1] == 0 { // For a highpass filter, cutoff[0] = fc, cutoff[1] = 0.
		// The filter must be a highpass filter.
		fd = cutoff[0] / float64(info.Rate)
		factor1 = -1
		factor2 = 1 - 2*fd
	} else {
		// Otherwise the filter must be a different type of filter.
		return nil, errors.New("tried to use newLoHiFilter to generate bandpass or bandstop filter")
	}

	// Create a new filter struct to return, populated with all correct data.
	var newFilter = SelectiveFrequencyFilter{cutoff: cutoff, sampleRate: info.Rate, taps: length, buffInfo: info}

	// Create a filter with characteristics from struct.
	size := newFilter.taps + 1
	newFilter.coeffs = make([]float64, size)
	b := 2 * math.Pi * fd
	winData := window.FlatTop(size)
	for n := 0; n < (newFilter.taps / 2); n++ {
		c := float64(n) - float64(newFilter.taps)/2
		y := math.Sin(c*b) / (math.Pi * c)
		newFilter.coeffs[n] = factor1 * y * winData[n]
		newFilter.coeffs[size-1-n] = newFilter.coeffs[n]
	}
	newFilter.coeffs[newFilter.taps/2] = factor2 * winData[newFilter.taps/2]

	// Return a pointer to the filter.
	return &newFilter, nil
}

// newBandFilter creates a ensures the validity of the input parameters, and generates appropriate lowpass and highpass filters
// required for the creation of the specific band filter.
func newBandFilter(cutoff [2]float64, info BufferFormat, length int) (new, lp, hp *SelectiveFrequencyFilter, err error) {
	// Ensure that all input values are valid.
	if cutoff[0] <= 0 || cutoff[0] >= float64(info.Rate)/2 {
		return nil, nil, nil, errors.New("cutoff frequencies out of bounds")
	} else if cutoff[1] <= 0 || cutoff[1] >= float64(info.Rate)/2 {
		return nil, nil, nil, errors.New("cutoff frequencies out of bounds")
	} else if length <= 0 {
		return nil, nil, nil, errors.New("cannot create filter with length <= 0")
	}
	// Create a new filter struct to return, populated with all correct data.
	// For a bandpass filter, cutoff[0] = fc_l, cutoff[1] = fc_u.
	// For a bandstop filter, cutoff[0] = fc_u, cutoff[1] = fc_l.
	var newFilter = SelectiveFrequencyFilter{cutoff: cutoff, sampleRate: info.Rate, taps: length, buffInfo: info}

	// Generate lowpass and highpass filters to create bandpass filter with.
	hp, err = NewHighPass(newFilter.cutoff[0], newFilter.buffInfo, newFilter.taps)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("could not create new highpass filter: %w", err)
	}
	lp, err = NewLowPass(newFilter.cutoff[1], newFilter.buffInfo, newFilter.taps)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("could not create new lowpass filter: %w", err)
	}

	// Return pointer to new filter.
	return &newFilter, hp, lp, nil
}

// convolveFromBytes takes in a byte slice and a float64 slice for a filter, converts to floats,
// convolves the two signals, and converts back to bytes and returns the convolution.
func convolveFromBytes(b []byte, filter []float64) ([]byte, error) {
	bufAsFloats, err := bytesToFloats(b)
	if err != nil {
		return nil, fmt.Errorf("could not convert to floats: %w", err)
	}

	// Convolve the floats with the filter.
	convolution, err := fastConvolve(bufAsFloats, filter)
	if err != nil {
		return nil, fmt.Errorf("could not compute fast convolution: %w", err)
	}
	outBytes, err := floatsToBytes(convolution)
	if err != nil {
		return nil, fmt.Errorf("could not convert convolution to bytes: %w", err)
	}
	return outBytes, nil
}

func bytesToFloats(b []byte) ([]float64, error) {
	// Ensure the validity of the input.
	if len(b) == 0 {
		return nil, errors.New("no audio to convert to floats")
	} else if len(b)%2 != 0 {
		return nil, errors.New("uneven number of bytes (not whole number of samples)")
	}

	// Convert bytes to floats.
	inputAsFloat := make([]float64, len(b)/2)
	inputAsInt := make([]int16, len(b)/2)
	bReader := bytes.NewReader(b)
	for i := range inputAsFloat {
		binary.Read(bReader, binary.LittleEndian, &inputAsInt[i])
		inputAsFloat[i] = float64(inputAsInt[i]) / (math.MaxInt16 + 1)
	}
	return inputAsFloat, nil
}

// floatsToBytes converts a slice of float64 PCM data into a slice of signed 16bit PCM data.
// The input float slice should contains values between -1 and 1. The function converts these values
// to a proportionate unsigned value between 0 and 65536. This 16bit integer is split into two bytes,
// then returned in Little Endian notation in a byte slice double the length of the input.
func floatsToBytes(f []float64) ([]byte, error) {
	buf := new(bytes.Buffer)
	bytes := make([]byte, len(f)*2)
	for i := range f {
		err := binary.Write(buf, binary.LittleEndian, int16(f[i]*math.MaxInt16))
		if err != nil {
			return nil, fmt.Errorf("failed to write ints as bytes: %w", err)
		}
	}
	n, err := buf.Read(bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to read bytes from buffer: %w", err)
	} else if n != len(bytes) {
		return nil, fmt.Errorf("buffer and output length mismatch read %d bytes, expected %d: %w", n, len(bytes), err)
	}

	return bytes, nil
}

// fastConvolve takes in a signal and an FIR filter and computes the convolution (runs in O(nlog(n)) time).
func fastConvolve(x, h []float64) ([]float64, error) {
	// Ensure valid data to convolve.
	if len(x) == 0 || len(h) == 0 {
		return nil, errors.New("convolution requires slice of length > 0")
	}

	// Calculate the length of the linear convolution.
	convLen := len(x) + len(h) - 1

	// Pad signals to the next largest power of 2 larger than convLen.
	padLen := int(math.Pow(2, math.Ceil(math.Log2(float64(convLen)))))
	zeros := make([]float64, padLen-len(x), padLen-len(h))
	x = append(x, zeros...)
	zeros = make([]float64, padLen-len(h))
	h = append(h, zeros...)

	// Compute DFFTs.
	x_fft, h_fft := fft.FFTReal(x), fft.FFTReal(h)

	// Compute the multiplication of the two signals in the freq domain.
	y_fft := make([]complex128, padLen)
	for i := range x_fft {
		y_fft[i] = x_fft[i] * h_fft[i]
	}

	// Compute the IDFFT.
	iy := fft.IFFT(y_fft)

	// Convert to []float64.
	y := make([]float64, padLen)
	for i := range iy {
		y[i] = real(iy[i])
	}

	// Trim to length of linear convolution and return.
	return y[0:convLen], nil
}
