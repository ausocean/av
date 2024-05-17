/*
NAME
  psi.go

DESCRIPTION
  See Readme.md

AUTHOR
  Saxon Milton <saxon@ausocean.org>

LICENSE
  Copyright (C) 2024 the Australian Ocean Lab (AusOcean). All Rights Reserved. 

  The Software and all intellectual property rights associated
  therewith, including but not limited to copyrights, trademarks,
  patents, and trade secrets, are and will remain the exclusive
  property of the Australian Ocean Lab (AusOcean).
*/


// Package psi provides encoding of MPEG-TS program specific information.
package psi

import (
	"errors"
	"fmt"

	"github.com/Comcast/gots/psi"
)

// PacketSize of psi (without MPEG-TS header)
const PacketSize = 184

// Lengths of section definitions.
const (
	ESSDataLen = 5
	DescDefLen = 2
	PMTDefLen  = 4
	PATLen     = 4
	TSSDefLen  = 5
	PSIDefLen  = 3
)

// Table Type IDs.
const (
	patID = 0x00
	pmtID = 0x02
)

// Consts relating to time description
// TODO: remove this, we don't do metadata like this anymore.
const (
	TimeDescTag  = 234
	TimeTagIndx  = 13
	TimeDataIndx = 15
	TimeDataSize = 8 // bytes, because time is stored in uint64
)

// Consts relating to location description
// TODO: remove this, we don't do metadata like this anymore.
const (
	LocationDescTag  = 235
	LocationTagIndx  = 23
	LocationDataIndx = 25
	LocationDataSize = 32 // bytes
)

// CRC hassh Size
const crcSize = 4

// Consts relating to syntax section.
const (
	TotalSyntaxSecLen = 180
	SyntaxSecLenIdx1  = 2
	SyntaxSecLenIdx2  = 3
	SyntaxSecLenMask1 = 0x03
	SectionLenMask1   = 0x03
)

// Consts relating to program info len.
const (
	ProgramInfoLenIdx1  = 11
	ProgramInfoLenIdx2  = 12
	ProgramInfoLenMask1 = 0x03
)

// DescriptorsIdx is the index that the descriptors start at.
const DescriptorsIdx = ProgramInfoLenIdx2 + 1

// MetadataTag is the descriptor tag used for metadata.
const MetadataTag = 0x26

// NewPATPSI will provide a standard program specific information (PSI) table
// with a program association table (PAT) specific data field.
func NewPATPSI() *PSI {
	return &PSI{
		PointerField:    0x00,
		TableID:         0x00,
		SyntaxIndicator: true,
		PrivateBit:      false,
		SectionLen:      0x0d,
		SyntaxSection: &SyntaxSection{
			TableIDExt:  0x01,
			Version:     0,
			CurrentNext: true,
			Section:     0,
			LastSection: 0,
			SpecificData: &PAT{
				Program:       0x01,
				ProgramMapPID: 0x1000,
			},
		},
	}
}

// NewPMTPSI will provide a standard program specific information (PSI) table
// with a program mapping table specific data field.
// NOTE: Media PID and stream ID are default to 0.
func NewPMTPSI() *PSI {
	return &PSI{
		PointerField:    0x00,
		TableID:         0x02,
		SyntaxIndicator: true,
		SectionLen:      0x12,
		SyntaxSection: &SyntaxSection{
			TableIDExt:  0x01,
			Version:     0,
			CurrentNext: true,
			Section:     0,
			LastSection: 0,
			SpecificData: &PMT{
				ProgramClockPID: 0x0100,
				ProgramInfoLen:  0,
				StreamSpecificData: &StreamSpecificData{
					StreamType:    0,
					PID:           0,
					StreamInfoLen: 0x00,
				},
			},
		},
	}
}

// TODO: get rid of these - not a good idea.
type (
	PSIBytes        []byte
	DescriptorBytes []byte
)

// Program specific information
type PSI struct {
	PointerField    byte           // Point field
	PointerFill     []byte         // Pointer filler bytes
	TableID         byte           // Table ID
	SyntaxIndicator bool           // Section syntax indicator (1 for PAT, PMT, CAT)
	PrivateBit      bool           // Private bit (0 for PAT, PMT, CAT)
	SectionLen      uint16         // Section length
	SyntaxSection   *SyntaxSection // Table syntax section (length defined by SectionLen) if length 0 then nil
	CRC             uint32         // crc32 of entire table excluding pointer field, pointer filler bytes and the trailing CRC32
}

// Table syntax section
type SyntaxSection struct {
	TableIDExt   uint16       // Table ID extension
	Version      byte         // Version number
	CurrentNext  bool         // Current/next indicator
	Section      byte         // Section number
	LastSection  byte         // Last section number
	SpecificData SpecificData // Specific data PAT/PMT
}

// Specific Data, (could be PAT or PMT)
type SpecificData interface {
	Bytes() []byte
}

// Program association table, implements SpecificData
type PAT struct {
	Program       uint16 // Program Number
	ProgramMapPID uint16 // Program map PID
}

// Program mapping table, implements SpecificData
type PMT struct {
	ProgramClockPID    uint16              // Program clock reference PID.
	ProgramInfoLen     uint16              // Program info length.
	Descriptors        []Descriptor        // Number of Program descriptors.
	StreamSpecificData *StreamSpecificData // Elementary stream specific data.
}

// Elementary stream specific data
type StreamSpecificData struct {
	StreamType    byte         // Stream type.
	PID           uint16       // Elementary PID.
	StreamInfoLen uint16       // Elementary stream info length.
	Descriptors   []Descriptor // Elementary stream desriptors
}

// Descriptor
type Descriptor struct {
	Tag  byte   // Descriptor tag
	Len  byte   // Descriptor length
	Data []byte // Descriptor data
}

// Bytes outputs a byte slice representation of the PSI
func (p *PSI) Bytes() []byte {
	out := make([]byte, 4)
	out[0] = p.PointerField
	if p.PointerField != 0 {
		panic("No support for pointer filler bytes")
	}
	out[1] = p.TableID
	out[2] = 0x80 | 0x30 | (0x03 & byte(p.SectionLen>>8))
	out[3] = byte(p.SectionLen)
	out = append(out, p.SyntaxSection.Bytes()...)
	out = AddCRC(out)
	return out
}

// Bytes outputs a byte slice representation of the SyntaxSection
func (t *SyntaxSection) Bytes() []byte {
	out := make([]byte, TSSDefLen)
	out[0] = byte(t.TableIDExt >> 8)
	out[1] = byte(t.TableIDExt)
	out[2] = 0xc0 | (0x3e & (t.Version << 1)) | (0x01 & asByte(t.CurrentNext))
	out[3] = t.Section
	out[4] = t.LastSection
	out = append(out, t.SpecificData.Bytes()...)
	return out
}

// Bytes outputs a byte slice representation of the PAT
func (p *PAT) Bytes() []byte {
	out := make([]byte, PATLen)
	out[0] = byte(p.Program >> 8)
	out[1] = byte(p.Program)
	out[2] = 0xe0 | (0x1f & byte(p.ProgramMapPID>>8))
	out[3] = byte(p.ProgramMapPID)
	return out
}

// Bytes outputs a byte slice representation of the PMT
func (p *PMT) Bytes() []byte {
	out := make([]byte, PMTDefLen)
	out[0] = 0xe0 | (0x1f & byte(p.ProgramClockPID>>8)) // byte 10
	out[1] = byte(p.ProgramClockPID)
	out[2] = 0xf0 | (0x03 & byte(p.ProgramInfoLen>>8))
	out[3] = byte(p.ProgramInfoLen)
	for _, d := range p.Descriptors {
		out = append(out, d.Bytes()...)
	}
	out = append(out, p.StreamSpecificData.Bytes()...)
	return out
}

// Bytes outputs a byte slice representation of the Desc
func (d *Descriptor) Bytes() []byte {
	out := make([]byte, DescDefLen)
	out[0] = d.Tag
	out[1] = d.Len
	out = append(out, d.Data...)
	return out
}

// Bytes outputs a byte slice representation of the StreamSpecificData
func (e *StreamSpecificData) Bytes() []byte {
	out := make([]byte, ESSDataLen)
	out[0] = e.StreamType
	out[1] = 0xe0 | (0x1f & byte(e.PID>>8))
	out[2] = byte(e.PID)
	out[3] = 0xf0 | (0x03 & byte(e.StreamInfoLen>>8))
	out[4] = byte(e.StreamInfoLen)
	for _, d := range e.Descriptors {
		out = append(out, d.Bytes()...)
	}
	return out
}

func asByte(b bool) byte {
	if b {
		return 0x01
	}
	return 0x00
}

// AddDescriptor adds or updates a descriptor in a PSI given a descriptor tag
// and data. If the psi is not a pmt, then an error is returned. If a descriptor
// with the given tag is not found in the psi, room is made and a descriptor with
// given tag and data is created. If a descriptor with the tag is found, the
// descriptor is resized as required and the new data is copied in.
func (p *PSIBytes) AddDescriptor(tag int, data []byte) error {
	if psi.TableID(*p) != pmtID {
		return errors.New("trying to add descriptor, but not pmt")
	}

	i, desc := p.HasDescriptor(tag)
	if desc == nil {
		err := p.createDescriptor(tag, data)
		if err != nil {
			return fmt.Errorf("could not create descriptor: %w", err)
		}
		return err
	}

	oldDescLen := desc.len()
	oldDataLen := int(desc[1])
	newDataLen := len(data)
	newDescLen := 2 + newDataLen
	delta := newDescLen - oldDescLen

	// If the old data length is more than the new data length, we need shift data
	// after descriptor up, and then trim the psi. If the oldDataLen is less than
	// new data then we need reseize psi and shift data down. If same do nothing.
	switch {
	case oldDataLen > newDataLen:
		copy((*p)[i+newDescLen:], (*p)[i+oldDescLen:])
		*p = (*p)[:len(*p)+delta]
	case oldDataLen < newDataLen:
		tmp := make([]byte, len(*p)+delta)
		copy(tmp, *p)
		*p = tmp
		copy((*p)[i+newDescLen:], (*p)[i+oldDescLen:])
	}

	// Copy in new data
	(*p)[i+1] = byte(newDataLen)
	copy((*p)[i+2:], data)

	newProgInfoLen := p.ProgramInfoLen() + delta
	p.setProgInfoLen(newProgInfoLen)
	newSectionLen := int(psi.SectionLength(*p)) + delta
	p.setSectionLen(newSectionLen)
	UpdateCrc((*p)[1:])
	return nil
}

// HasDescriptor checks if a descriptor of the given tag exists in a PSI. If the descriptor
// of the given tag exists, an index of this descriptor, as well as the Descriptor is returned.
// If the descriptor of the given tag cannot be found, -1 and a nil slice is returned.
//
// TODO: check if pmt, return error if not ?
func (p *PSIBytes) HasDescriptor(tag int) (int, DescriptorBytes) {
	descs := p.descriptors()
	if descs == nil {
		return -1, nil
	}
	for i := 0; i < len(descs); i += 2 + int(descs[i+1]) {
		if int(descs[i]) == tag {
			return i + DescriptorsIdx, descs[i : i+2+int(descs[i+1])]
		}
	}
	return -1, nil
}

// createDescriptor creates a descriptor in a psi given a tag and data. It does so
// by resizing the psi, shifting existing data down and copying in new descriptor
// in new space.
func (p *PSIBytes) createDescriptor(tag int, data []byte) error {
	curProgLen := p.ProgramInfoLen()
	oldSyntaxSectionLen := SyntaxSecLenFrom(*p)
	if TotalSyntaxSecLen-(oldSyntaxSectionLen+2+len(data)) <= 0 {
		return errors.New("Not enough space in psi to create descriptor.")
	}
	dataLen := len(data)
	newDescIdx := DescriptorsIdx + curProgLen
	newDescLen := dataLen + 2

	// Increase size of psi and copy data down to make room for new descriptor.
	tmp := make([]byte, len(*p)+newDescLen)
	copy(tmp, *p)
	*p = tmp
	copy((*p)[newDescIdx+newDescLen:], (*p)[newDescIdx:newDescIdx+newDescLen])
	// Set the tag, data len and data of the new desriptor.
	(*p)[newDescIdx] = byte(tag)
	(*p)[newDescIdx+1] = byte(dataLen)
	copy((*p)[newDescIdx+2:newDescIdx+2+dataLen], data)

	// Set length fields and update the psi CRC.
	addedLen := dataLen + 2
	newProgInfoLen := curProgLen + addedLen
	p.setProgInfoLen(newProgInfoLen)
	newSyntaxSectionLen := int(oldSyntaxSectionLen) + addedLen
	p.setSectionLen(newSyntaxSectionLen)
	UpdateCrc((*p)[1:])

	return nil
}

// setProgInfoLen sets the program information length in a psi with a pmt.
func (p *PSIBytes) setProgInfoLen(l int) {
	(*p)[ProgramInfoLenIdx1] &= 0xff ^ ProgramInfoLenMask1
	(*p)[ProgramInfoLenIdx1] |= byte(l>>8) & ProgramInfoLenMask1
	(*p)[ProgramInfoLenIdx2] = byte(l)
}

// setSectionLen sets section length in a psi.
func (p *PSIBytes) setSectionLen(l int) {
	(*p)[SyntaxSecLenIdx1] &= 0xff ^ SyntaxSecLenMask1
	(*p)[SyntaxSecLenIdx1] |= byte(l>>8) & SyntaxSecLenMask1
	(*p)[SyntaxSecLenIdx2] = byte(l)
}

// descriptors returns the descriptors in a psi if they exist, otherwise
// a nil slice is returned.
func (p *PSIBytes) descriptors() []byte {
	return (*p)[DescriptorsIdx : DescriptorsIdx+p.ProgramInfoLen()]
}

// len returns the length of a descriptor in bytes.
func (d *DescriptorBytes) len() int {
	return int(2 + (*d)[1])
}

// ProgramInfoLen returns the program info length of a PSI.
//
// TODO: check if pmt - if not return 0 ? or -1 ?
func (p *PSIBytes) ProgramInfoLen() int {
	return int((((*p)[ProgramInfoLenIdx1] & ProgramInfoLenMask1) << 8) | (*p)[ProgramInfoLenIdx2])
}
