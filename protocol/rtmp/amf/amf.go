/*
NAME
  amf.go

DESCRIPTION
  Action Message Format (AMF) encoding/decoding functions.
  See https://en.wikipedia.org/wiki/Action_Message_Format.

AUTHORS
  Saxon Nelson-Milton <saxon@ausocean.org>
  Dan Kortschak <dan@ausocean.org>
  Jake Lane <jake@ausocean.org>
  Alan Noble <alan@ausocean.org>

LICENSE
  Copyright (C) 2024 the Australian Ocean Lab (AusOcean). All Rights Reserved. 

  The Software and all intellectual property rights associated
  therewith, including but not limited to copyrights, trademarks,
  patents, and trade secrets, are and will remain the exclusive
  property of the Australian Ocean Lab (AusOcean).
*/


// Package amf implements Action Message Format (AMF) encoding and decoding.
// In AMF, encoding of numbers is big endian by default, unless specified otherwise,
// and numbers are all unsigned.
// See https://en.wikipedia.org/wiki/Action_Message_Format.
package amf

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"
)

// AMF data types, as defined by the AMF specification.
// NB: we export these sparingly.
const (
	typeNumber      = 0x00
	typeBoolean     = 0x01
	TypeString      = 0x02
	TypeObject      = 0x03
	typeMovieClip   = 0x04
	TypeNull        = 0x05
	typeUndefined   = 0x06
	typeReference   = 0x07
	typeEcmaArray   = 0x08
	TypeObjectEnd   = 0x09
	typeStrictArray = 0x0A
	typeDate        = 0x0B
	typeLongString  = 0x0C
	typeUnsupported = 0x0D
	typeRecordset   = 0x0E
	typeXmlDoc      = 0x0F
	typeTypedObject = 0x10
	typeAvmplus     = 0x11
	typeInvalid     = 0xff
)

// AMF represents an AMF object, which is simply a collection of properties.
type Object struct {
	Properties []Property
}

// Property represents an AMF property, which is effectively a
// union. The Type is the AMF data type (uint8 per the specification),
// and specifies which member holds a value.  Numeric types use
// Number, string types use String and arrays and objects use
// Object. The Name is optional.
type Property struct {
	Type uint8

	Name   string
	Number float64
	String string
	Object Object
}

// AMF errors:
var (
	ErrShortBuffer      = errors.New("amf: short buffer")       // The supplied buffer was too short.
	ErrInvalidType      = errors.New("amf: invalid type")       // An invalid type was supplied to the encoder.
	ErrUnexpectedType   = errors.New("amf: unexpected type")    // An unexpected type was encountered while decoding.
	ErrPropertyNotFound = errors.New("amf: property not found") // The requested property was not found.
)

// DecodeInt16 decodes a 16-bit integer.
func DecodeInt16(buf []byte) uint16 {
	return binary.BigEndian.Uint16(buf)
}

// DecodeInt24 decodes a 24-bit integer.
func DecodeInt24(buf []byte) uint32 {
	return uint32(buf[0])<<16 | uint32(buf[1])<<8 | uint32(buf[2])
}

// DecodeInt32 decodes a 32-bit integer.
func DecodeInt32(buf []byte) uint32 {
	return binary.BigEndian.Uint32(buf)
}

// DecodeInt32LE decodes a 32-bit little-endian integer.
func DecodeInt32LE(buf []byte) uint32 {
	return binary.LittleEndian.Uint32(buf)
}

// DecodeString decodes a string that is less than 2^16 bytes long.
func DecodeString(buf []byte) string {
	n := DecodeInt16(buf)
	return string(buf[2 : 2+n])
}

// DecodeLongString decodes a long string.
func DecodeLongString(buf []byte) string {
	n := DecodeInt32(buf)
	return string(buf[2 : 2+n])
}

// DecodeNumber decodes a 64-bit floating-point number.
func DecodeNumber(buf []byte) float64 {
	return math.Float64frombits(binary.BigEndian.Uint64(buf))
}

// DecodeBoolean decodes a boolean.
func DecodeBoolean(buf []byte) bool {
	return buf[0] != 0
}

// EncodeInt24 encodes a 24-bit integer.
func EncodeInt24(buf []byte, val uint32) ([]byte, error) {
	if len(buf) < 3 {
		return nil, ErrShortBuffer
	}
	buf[0] = byte(val >> 16)
	buf[1] = byte(val >> 8)
	buf[2] = byte(val)
	return buf[3:], nil
}

// EncodeInt32 encodes a 32-bit integer.
func EncodeInt32(buf []byte, val uint32) ([]byte, error) {
	if len(buf) < 4 {
		return nil, ErrShortBuffer
	}
	binary.BigEndian.PutUint32(buf, val)
	return buf[4:], nil
}

// EncodeString encodes a string.
// Strings less than 65536 in length are encoded as TypeString, while longer strings are ecodeded as typeLongString.
func EncodeString(buf []byte, val string) ([]byte, error) {
	const typeSize = 1
	if len(val) < 65536 && len(val)+typeSize+binary.Size(int16(0)) > len(buf) || len(val)+typeSize+binary.Size(uint32(0)) > len(buf) {
		return nil, ErrShortBuffer
	}

	if len(val) < 65536 {
		buf[0] = TypeString
		buf = buf[1:]
		binary.BigEndian.PutUint16(buf[:2], uint16(len(val)))
		buf = buf[2:]
		copy(buf, val)
		return buf[len(val):], nil
	}

	buf[0] = typeLongString
	buf = buf[1:]
	binary.BigEndian.PutUint32(buf[:4], uint32(len(val)))
	buf = buf[4:]
	copy(buf, val)
	return buf[len(val):], nil
}

// EncodeNumber encodes a 64-bit floating-point number.
func EncodeNumber(buf []byte, val float64) ([]byte, error) {
	if len(buf) < 9 {
		return nil, ErrShortBuffer
	}
	buf[0] = typeNumber
	buf = buf[1:]
	binary.BigEndian.PutUint64(buf, math.Float64bits(val))
	return buf[8:], nil
}

// EncodeBoolean encodes a boolean.
func EncodeBoolean(buf []byte, val bool) ([]byte, error) {
	if len(buf) < 2 {
		return nil, ErrShortBuffer
	}
	buf[0] = typeBoolean
	if val {
		buf[1] = 1
	} else {
		buf[1] = 0
	}
	return buf[2:], nil

}

// EncodeNamedString encodes a named string, where key is the name and val is the string value.
func EncodeNamedString(buf []byte, key, val string) ([]byte, error) {
	if 2+len(key) > len(buf) {
		return nil, ErrShortBuffer
	}
	binary.BigEndian.PutUint16(buf[:2], uint16(len(key)))
	buf = buf[2:]
	copy(buf, key)
	b, err := EncodeString(buf[len(key):], val)
	if err != nil {
		return nil, fmt.Errorf("could not encode string: %w", err)
	}
	return b, nil
}

// EncodeNamedNumber encodes a named number, where key is the name and val is the number value.
func EncodeNamedNumber(buf []byte, key string, val float64) ([]byte, error) {
	if 2+len(key) > len(buf) {
		return nil, ErrShortBuffer
	}
	binary.BigEndian.PutUint16(buf[:2], uint16(len(key)))
	buf = buf[2:]
	copy(buf, key)
	b, err := EncodeNumber(buf[len(key):], val)
	if err != nil {
		return nil, fmt.Errorf("could not encode number: %w", err)
	}
	return b, nil
}

// EncodeNamedNumber encodes a named boolean, where key is the name and val is the boolean value.
func EncodeNamedBoolean(buf []byte, key string, val bool) ([]byte, error) {
	if 2+len(key) > len(buf) {
		return nil, ErrShortBuffer
	}
	binary.BigEndian.PutUint16(buf[:2], uint16(len(key)))
	buf = buf[2:]
	copy(buf, key)
	b, err := EncodeBoolean(buf[len(key):], val)
	if err != nil {
		return nil, fmt.Errorf("could not encode boolean: %w", err)
	}
	return b, nil
}

// EncodeProperty encodes a property.
func EncodeProperty(prop *Property, buf []byte) ([]byte, error) {
	if prop.Type != TypeNull && prop.Name != "" {
		if len(buf) < 2+len(prop.Name) {
			return nil, fmt.Errorf("not type null, short buffer: %w", ErrShortBuffer)
		}
		binary.BigEndian.PutUint16(buf[:2], uint16(len(prop.Name)))
		buf = buf[2:]
		copy(buf, prop.Name)
		buf = buf[len(prop.Name):]
	}

	switch prop.Type {
	case typeNumber:
		b, err := EncodeNumber(buf, prop.Number)
		if err != nil {
			return nil, fmt.Errorf("could not encode number: %w", err)
		}
		return b, nil
	case typeBoolean:
		b, err := EncodeBoolean(buf, prop.Number != 0)
		if err != nil {
			return nil, fmt.Errorf("could not encode boolean: %w", err)
		}
		return b, nil
	case TypeString:
		b, err := EncodeString(buf, prop.String)
		if err != nil {
			return nil, fmt.Errorf("could not encode string: %w", err)
		}
		return b, nil
	case TypeNull:
		if len(buf) < 2 {
			return nil, fmt.Errorf("type null, short buffer: %w", ErrShortBuffer)
		}
		buf[0] = TypeNull
		buf = buf[1:]
	case TypeObject:
		b, err := Encode(&prop.Object, buf)
		if err != nil {
			return nil, fmt.Errorf("could not encode: %w", err)
		}
		return b, nil
	case typeEcmaArray:
		b, err := EncodeEcmaArray(&prop.Object, buf)
		if err != nil {
			return nil, fmt.Errorf("could not encode ecma array: %w", err)
		}
		return b, nil
	case typeStrictArray:
		b, err := EncodeArray(&prop.Object, buf)
		if err != nil {
			return nil, fmt.Errorf("could not encode array: %w", err)
		}
		return b, nil
	default:
		return nil, ErrInvalidType
	}
	return buf, nil
}

// DecodeProperty decodes a property, returning the number of bytes consumed from the supplied buffer.
func DecodeProperty(prop *Property, buf []byte, decodeName bool) (int, error) {
	sz := len(buf)

	if decodeName {
		if len(buf) < 4 {
			return 0, ErrShortBuffer
		}
		n := DecodeInt16(buf[:2])
		if int(n) > len(buf)-2 {
			return 0, fmt.Errorf("short buffer after decode of int 16: %w", ErrShortBuffer)
		}

		prop.Name = DecodeString(buf)
		buf = buf[2+n:]
	} else {
		prop.Name = ""
	}

	prop.Type = buf[0]
	buf = buf[1:]

	switch prop.Type {
	case typeNumber:
		if len(buf) < 8 {
			return 0, fmt.Errorf("type number short buffer: %w", ErrShortBuffer)
		}
		prop.Number = DecodeNumber(buf[:8])
		buf = buf[8:]

	case typeBoolean:
		if len(buf) < 1 {
			return 0, fmt.Errorf("type boolean short buffer: %w", ErrShortBuffer)
		}
		prop.Number = float64(buf[0])
		buf = buf[1:]

	case TypeString:
		n := DecodeInt16(buf[:2])
		if len(buf) < int(n+2) {
			return 0, fmt.Errorf("type string: %w", ErrShortBuffer)
		}
		prop.String = DecodeString(buf)
		buf = buf[2+n:]

	case TypeObject:
		n, err := Decode(&prop.Object, buf, true)
		if err != nil {
			return 0, fmt.Errorf("could not decode type object: %w", err)
		}
		buf = buf[n:]

	case TypeNull, typeUndefined, typeUnsupported:
		prop.Type = TypeNull

	case typeEcmaArray:
		buf = buf[4:]
		n, err := Decode(&prop.Object, buf, true)
		if err != nil {
			return 0, fmt.Errorf("could not decode type ecma array: %w", err)
		}
		buf = buf[n:]

	default:
		return 0, ErrUnexpectedType
	}

	return sz - len(buf), nil
}

// Encode encodes an Object into its AMF representation.
func Encode(obj *Object, buf []byte) ([]byte, error) {
	if len(buf) < 5 {
		return nil, ErrShortBuffer
	}

	buf[0] = TypeObject
	buf = buf[1:]

	for i := 0; i < len(obj.Properties); i++ {
		var err error
		buf, err = EncodeProperty(&obj.Properties[i], buf)
		if err != nil {
			return nil, fmt.Errorf("could not encode property no. %d: %w", i, err)
		}
	}

	if len(buf) < 3 {
		return nil, fmt.Errorf("short buffer after property encoding: %w", ErrShortBuffer)
	}

	b, err := EncodeInt24(buf, TypeObjectEnd)
	if err != nil {
		return nil, fmt.Errorf("could not encode int 24: %w", err)
	}
	return b, err
}

// EncodeEcmaArray encodes an ECMA array.
func EncodeEcmaArray(obj *Object, buf []byte) ([]byte, error) {
	if len(buf) < 5 {
		return nil, ErrShortBuffer
	}

	buf[0] = typeEcmaArray
	buf = buf[1:]
	binary.BigEndian.PutUint32(buf[:4], uint32(len(obj.Properties)))
	buf = buf[4:]

	for i := 0; i < len(obj.Properties); i++ {
		var err error
		buf, err = EncodeProperty(&obj.Properties[i], buf)
		if err != nil {
			return nil, fmt.Errorf("could not encode property no. %d: %w", i, err)
		}
	}

	if len(buf) < 3 {
		return nil, fmt.Errorf("short buffer after property encoding: %w", ErrShortBuffer)
	}

	b, err := EncodeInt24(buf, TypeObjectEnd)
	if err != nil {
		return nil, fmt.Errorf("could not encode int 24: %w", err)
	}

	return b, nil
}

// EncodeArray encodes an array.
func EncodeArray(obj *Object, buf []byte) ([]byte, error) {
	if len(buf) < 5 {
		return nil, ErrShortBuffer
	}

	buf[0] = typeStrictArray
	buf = buf[1:]
	binary.BigEndian.PutUint32(buf[:4], uint32(len(obj.Properties)))
	buf = buf[4:]

	for i := 0; i < len(obj.Properties); i++ {
		var err error
		buf, err = EncodeProperty(&obj.Properties[i], buf)
		if err != nil {
			return nil, fmt.Errorf("could not encode property no. %d: %w", i, err)
		}
	}

	return buf, nil
}

// Decode decodes an object. Property names are only decoded if decodeName is true.
func Decode(obj *Object, buf []byte, decodeName bool) (int, error) {
	sz := len(buf)

	obj.Properties = obj.Properties[:0]
	for len(buf) != 0 {
		if len(buf) >= 3 && DecodeInt24(buf[:3]) == TypeObjectEnd {
			buf = buf[3:]
			break
		}
		var prop Property
		n, err := DecodeProperty(&prop, buf, decodeName)
		if err != nil {
			return 0, fmt.Errorf("could not decode property: %w", err)
		}
		buf = buf[n:]
		obj.Properties = append(obj.Properties, prop)
	}

	return sz - len(buf), nil
}

// Object methods:

// Property returns a property, either by its index when idx is non-negative, or by its name otherwise.
// If the requested property is not found or the type does not match, an ErrPropertyNotFound error is returned.
func (obj *Object) Property(name string, idx int, typ uint8) (*Property, error) {
	var prop *Property
	if idx >= 0 {
		if idx < len(obj.Properties) {
			prop = &obj.Properties[idx]
		}
	} else {
		for i, p := range obj.Properties {
			if p.Name == name {
				prop = &obj.Properties[i]
				break
			}
		}
	}
	if prop == nil || prop.Type != typ {
		return nil, ErrPropertyNotFound
	}
	return prop, nil
}

// NumberProperty is a wrapper for Property that returns a Number property's value, if any.
func (obj *Object) NumberProperty(name string, idx int) (float64, error) {
	prop, err := obj.Property(name, idx, typeNumber)
	if err != nil {
		return 0, err
	}
	return prop.Number, nil
}

// StringProperty is a wrapper for Property that returns a String property's  value, if any.
func (obj *Object) StringProperty(name string, idx int) (string, error) {
	prop, err := obj.Property(name, idx, TypeString)
	if err != nil {
		return "", err
	}
	return prop.String, nil
}

// ObjectProperty is a wrapper for Property that returns an Object property's value, if any.
func (obj *Object) ObjectProperty(name string, idx int) (*Object, error) {
	prop, err := obj.Property(name, idx, TypeObject)
	if err != nil {
		return nil, err
	}
	return &prop.Object, nil
}
