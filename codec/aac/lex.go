package codecutil

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

// ADTSHeader holds the parsed fields of an ADTS frame header.
type ADTSHeader struct {
	// Fixed Header Fields
	Syncword               uint16 // Should always be 0xFFF
	MPEGID                 uint8  // 0: MPEG-4, 1: MPEG-2
	ProtectionAbsent       bool   // true if no CRC (7-byte header)
	Profile                uint8  // AAC Profile (1=AAC-LC)
	SamplingFrequencyIndex uint8
	ChannelConfiguration   uint8

	// Variable Header Fields
	FrameLength   uint16 // Total length of this ADTS frame in bytes (header + payload)
	RawDataBlocks uint8  // Number of raw data blocks (payloads) in this frame minus 1 (usually 0)
}

// Fixed constant for the Syncword (1111 1111 1111)
const adtsSyncword uint16 = 0xFFF

// ADTS_HEADER_SIZE is 7 bytes if protection_absent is 1 (no CRC).
const ADTS_HEADER_SIZE = 7

// ReadADTSFrame reads the next ADTS frame from the stream and returns the parsed header, a byte slice containing the payload, and any errors that occurred.
func ReadADTSFrame(r io.Reader) (*ADTSHeader, []byte, error) {
	buf := make([]byte, ADTS_HEADER_SIZE)

	// 1. Read the 7-byte header block
	n, err := io.ReadFull(r, buf)
	if err != nil {
		if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
			return nil, nil, io.EOF
		}
		return nil, nil, fmt.Errorf("failed to read ADTS header: %w", err)
	}
	if n < ADTS_HEADER_SIZE {
		return nil, nil, io.ErrUnexpectedEOF
	}

	header := &ADTSHeader{}

	// ADTS header is Big Endian, and fields often cross byte boundaries.
	// We use the full 32-bit representation of the first 4 bytes for easy bit masking.
	fixedHeader := binary.BigEndian.Uint32(buf[0:4])

	// --- 2. Fixed Header Parsing (Bits 0 - 27) ---

	// Syncword (12 bits) - Bits 20-31 (0xFFF00000 >> 20)
	// Masking: 0xFFF00000, then shifting right 20 bits
	header.Syncword = uint16((fixedHeader & 0xFFF00000) >> 20)
	if header.Syncword != adtsSyncword {
		return nil, nil, fmt.Errorf("syncword mismatch: expected 0x%X, got 0x%X (0x%X)", adtsSyncword, header.Syncword, fixedHeader)
	}

	// ID (1 bit) - Bit 19
	header.MPEGID = uint8((fixedHeader & 0x00080000) >> 19)

	// Layer (2 bits) - Bits 17-18. Always 00. Skipped for simplicity.

	// Protection Absent (1 bit) - Bit 16
	// If 1, no CRC (7-byte header assumed). If 0, CRC is present (9-byte header).
	header.ProtectionAbsent = (fixedHeader&0x00010000)>>16 == 1

	// Profile (2 bits) - Bits 14-15
	header.Profile = uint8((fixedHeader & 0x00006000) >> 14) // Profile is 2 bits, but shifted 13 from right

	// Sampling Frequency Index (4 bits) - Bits 10-13
	header.SamplingFrequencyIndex = uint8((fixedHeader & 0x00001E00) >> 10) // Index is 4 bits, but shifted 9 from right

	// Private bit, Original/Copy, Home bits skipped for simplicity.

	// Channel Configuration (3 bits) - Bits 1-3
	// This field straddles the 3rd and 4th byte. We read the remaining 4 bits from the fixedHeader (bits 0-3)
	// and the first 3 bits of the Variable Header (bits 4-6 of the 4th byte).

	// The 3 channel bits are: (2 bits from Byte 3) + (1 bit from Byte 4)
	// Correct parsing requires looking at the raw bytes:
	// Byte 3: ... | Freq Index | Private Bit (1) | Channel MSB (2 bits) |
	// Byte 4: Channel LSB (1 bit) | Original/Copy (1) | Home (1) | ...

	// Let's re-parse Channel Configuration more carefully using Byte 3 and Byte 4 directly:
	// Byte 3 is buf[2], Byte 4 is buf[3]

	// Bits 0-2 (3 bits)
	channelConfigBits := (buf[2] & 0x01) << 2 // MSB of channel config
	channelConfigBits |= (buf[3] & 0xC0) >> 6 // Next 2 bits of channel config

	header.ChannelConfiguration = uint8(channelConfigBits)

	// --- 3. Variable Header Parsing (Partial) ---
	// The next 4 bytes are required for the variable fields.

	// Frame Length (13 bits) - Bits 25-13 (STRADDLES BYTE 3, 4, 5)
	// We need 4 bits from Byte 3, all 8 bits from Byte 4, and 1 bit from Byte 5.

	// Byte 3 (last 2 bits, 2 bits) -> MSBs
	varFrameLength := uint16(buf[3]&0x0F) << 11
	// Byte 4 (all 8 bits, 8 bits) -> Middle bits
	varFrameLength |= uint16(buf[4]) << 3
	// Byte 5 (first 3 bits, 3 bit) -> LSB
	varFrameLength |= uint16(buf[5]&0xE0) >> 5

	header.FrameLength = varFrameLength

	// Raw Data Blocks (2 bits) - Bits 45-46
	// The last two bits of the header are RawDataBlocks:
	header.RawDataBlocks = uint8(buf[6] & 0x03)

	// --- 4. Read the Raw AAC Payload ---

	// Check for a valid frame length (must be at least the size of the header)
	if header.FrameLength < ADTS_HEADER_SIZE {
		return header, nil, fmt.Errorf("invalid frame length: %d bytes (less than header size %d)", header.FrameLength, ADTS_HEADER_SIZE)
	}

	// Calculate the size of the payload (the raw AAC frame)
	payloadSize := int(header.FrameLength) - ADTS_HEADER_SIZE

	// If protection is NOT absent, we need to account for the 16-bit CRC
	if !header.ProtectionAbsent {
		payloadSize -= 2 // Subtract 2 bytes for the CRC checksum
	}

	if payloadSize <= 0 {
		return header, nil, errors.New("calculated payload size is zero or negative")
	}

	// Create a buffer for the raw AAC payload
	payloadBuf := make([]byte, payloadSize)

	// Read the payload data from the stream
	_, err = io.ReadFull(r, payloadBuf)
	if err != nil {
		return header, nil, fmt.Errorf("failed to read frame payload of size %d: %w", payloadSize, err)
	}

	// If protection is present (9-byte header), we must skip the 16-bit CRC that follows the payload.
	// We read it but don't store it in the payloadBuf.
	if !header.ProtectionAbsent {
		_, err = io.CopyN(io.Discard, r, 2)
		if err != nil {
			return header, nil, fmt.Errorf("failed to skip CRC checksum: %w", err)
		}
	}

	return header, payloadBuf, nil
}

// ADTSHeaderToAudioSpecificConfig converts the relevant fields from a parsed
// ADTSHeader into the raw binary AudioSpecificConfig (ASC) byte slice.
func ADTSHeaderToAudioSpecificConfig(header *ADTSHeader) ([]byte, error) {
	// The ADTS profile index (2 bits) must be mapped to the ASC audioObjectType (5 bits).
	// This mapping assumes common AAC-LC streams.
	var audioObjectType uint8
	switch header.Profile {
	case 1: // AAC-LC (Low Complexity) is ADTS Profile 1
		audioObjectType = 2 // AAC-LC is ASC Object Type 2
	case 2: // SBR/HE-AAC (if present, usually signaled via extension)
		audioObjectType = 5 // SBR/HE-AAC is ASC Object Type 5
	default:
		// For other profiles, we map the 2-bit ADTS profile directly to the
		// lower 2 bits of the 5-bit ASC object type, which is usually sufficient
		// for standard profiles like Main (0) and LTP (3).
		audioObjectType = header.Profile
	}

	if audioObjectType > 31 {
		// If the object type is high, the ASC format includes an 'escape' value and more bits.
		// For standard ADTS conversion, this is generally not encountered.
		return nil, fmt.Errorf("unsupported or complex audio object type derived from ADTS profile: %d", header.Profile)
	}

	var configWord uint16

	// 1. audioObjectType (5 bits) -> Bits 11-15 (MSB)
	// Shifts: 5 bits for object type + 4 bits for freq index + 4 bits for channel config = 13 bits.
	// We want to store it in the top 5 bits of the 16-bit word.
	configWord |= uint16(audioObjectType) << 11

	// 2. samplingFrequencyIndex (4 bits) -> Bits 7-10
	configWord |= uint16(header.SamplingFrequencyIndex) << 7

	// 3. channelConfiguration (4 bits) -> Bits 3-6
	// The ADTS ChannelConfiguration is 3 bits, but the ASC field is 4 bits.
	// For standard channels (1-6), the 3 bits map directly.
	configWord |= uint16(header.ChannelConfiguration) << 3

	// The remaining 3 bits (Bits 0-2) are often reserved or zeroed out
	// for simple AAC-LC streams.

	// Convert the 16-bit word into a 2-byte Big Endian slice.
	config := make([]byte, 2)
	config[0] = byte(configWord >> 8)
	config[1] = byte(configWord & 0xFF)

	return config, nil
}
