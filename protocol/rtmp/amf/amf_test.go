/*
NAME
  amf_test.go

DESCRIPTION
  AMF test suite.

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


package amf

import (
	"testing"
)

// Test data.
var testStrings = [...]string{
	"",
	"foo",
	"bar",
	"bazz",
}

var testNumbers = [...]uint32{
	0,
	1,
	0xababab,
	0xffffff,
}

// TestSanity checks that we haven't accidentally changed constants.
func TestSanity(t *testing.T) {
	if TypeObjectEnd != 0x09 {
		t.Errorf("TypeObjectEnd has wrong value; got %d, expected %d", TypeObjectEnd, 0x09)
	}
}

// TestStrings tests string encoding and decoding.
func TestStrings(t *testing.T) {
	// Short string encoding is as follows:
	//   enc[0]   = data type (TypeString)
	//   end[1:3] = size
	//   enc[3:]  = data
	for _, s := range testStrings {
		buf := make([]byte, len(s)+7)
		_, err := EncodeString(buf, s)
		if err != nil {
			t.Errorf("EncodeString failed")
		}
		if buf[0] != TypeString {
			t.Errorf("Expected TypeString, got %v", buf[0])
		}
		ds := DecodeString(buf[1:])
		if s != ds {
			t.Errorf("DecodeString did not produce original string, got %v", ds)
		}
	}
	// Long string encoding is as follows:
	//   enc[0]   = data type (TypeString)
	//   end[1:5] = size
	//   enc[5:]  = data
	s := string(make([]byte, 65536))
	buf := make([]byte, len(s)+7)
	_, err := EncodeString(buf, s)
	if err != nil {
		t.Errorf("EncodeString failed")
	}
	if buf[0] != typeLongString {
		t.Errorf("Expected typeLongString, got %v", buf[0])
	}
	ds := DecodeLongString(buf[1:])
	if s != ds {
		t.Errorf("DecodeLongString did not produce original string, got %v", ds)
	}
}

// TestNumbers tests number encoding and encoding.
func TestNumbers(t *testing.T) {
	for _, n := range testNumbers {
		buf := make([]byte, 4) // NB: encoder requires an extra byte for some reason
		_, err := EncodeInt24(buf, n)
		if err != nil {
			t.Errorf("EncodeInt24 failed")
		}
		dn := DecodeInt24(buf)
		if n != dn {
			t.Errorf("DecodeInt24 did not produce original Number, got %v", dn)
		}
		_, err = EncodeInt32(buf, n)
		if err != nil {
			t.Errorf("EncodeInt32 failed")
		}
		dn = DecodeInt32(buf)
		if n != dn {
			t.Errorf("DecodeInt32 did not produce original Number, got %v", dn)
		}
	}
}

// TestProperties tests encoding and decoding of properties.
func TestProperties(t *testing.T) {
	var buf [1024]byte

	// Encode/decode Number properties.
	enc := buf[:]
	var err error
	for i := range testNumbers {
		enc, err = EncodeProperty(&Property{Type: typeNumber, Number: float64(testNumbers[i])}, enc)
		if err != nil {
			t.Errorf("EncodeProperty of Number failed")
		}

	}

	prop := Property{}
	dec := buf[:]
	for i := range testNumbers {
		n, err := DecodeProperty(&prop, dec, false)
		if err != nil {
			t.Errorf("DecodeProperty of Number failed")
		}
		if prop.Number != float64(testNumbers[i]) {
			t.Errorf("EncodeProperty/DecodeProperty returned wrong Number; got %v, expected %v", int32(prop.Number), testNumbers[i])
		}
		dec = dec[n:]
	}

	// Encode/decode string properties.
	enc = buf[:]
	for i := range testStrings {
		enc, err = EncodeProperty(&Property{Type: TypeString, String: testStrings[i]}, enc)
		if err != nil {
			t.Errorf("EncodeProperty of string failed")
		}

	}
	prop = Property{}
	dec = buf[:]
	for i := range testStrings {
		n, err := DecodeProperty(&prop, dec, false)
		if err != nil {
			t.Errorf("DecodeProperty of string failed")
		}
		if prop.String != testStrings[i] {
			t.Errorf("EncodeProperty/DecodeProperty returned wrong string; got %s, expected %s", prop.String, testStrings[i])
		}
		dec = dec[n:]
	}

}

// TestObject tests encoding and decoding of objects.
func TestObject(t *testing.T) {
	var buf [1024]byte

	// Construct a simple object that has one property, the Number 42.
	prop1 := Property{Type: typeNumber, Number: 42}
	obj1 := Object{}
	obj1.Properties = append(obj1.Properties, prop1)

	// Encode it
	enc := buf[:]
	var err error
	enc, err = Encode(&obj1, enc)
	if err != nil {
		t.Errorf("Encode of object failed")
	}

	// Check the encoding
	if buf[0] != TypeObject {
		t.Errorf("Encoded wrong type; expected %d, got %v", TypeObject, buf[0])
	}
	if buf[1] != typeNumber {
		t.Errorf("Encoded wrong type; expected %d, got %v", typeNumber, buf[0])
	}
	num := DecodeNumber(buf[2:10])
	if num != 42 {
		t.Errorf("Encoded wrong Number")
	}
	end := int32(DecodeInt24(buf[10:13]))
	if end != TypeObjectEnd {
		t.Errorf("Did not encode TypeObjectEnd")
	}

	// Decode it
	dec := buf[1:]
	var dobj1 Object
	_, err = Decode(&dobj1, dec, false)
	if err != nil {
		t.Errorf("Decode of object failed")
	}

	// Change the object's property to a named boolean.
	obj1.Properties[0].Name = "on"
	obj1.Properties[0].Type = typeBoolean
	obj1.Properties[0].Number = 1

	// Re-encode it
	enc = buf[:]
	enc, err = Encode(&obj1, enc)
	if err != nil {
		t.Errorf("Encode of object failed")
	}

	// Decode it, this time with named set to true.
	dec = buf[1:]
	_, err = Decode(&dobj1, dec, true)
	if err != nil {
		t.Errorf("Decode of object failed with error: %v", err)
	}
	if dobj1.Properties[0].Number != 1 {
		t.Errorf("Decoded wrong boolean value")
	}

	// Construct a more complicated object that includes a nested object.
	var obj2 Object
	for i := range testStrings {
		obj2.Properties = append(obj2.Properties, Property{Type: TypeString, String: testStrings[i]})
		obj2.Properties = append(obj2.Properties, Property{Type: typeNumber, Number: float64(testNumbers[i])})
	}
	obj2.Properties = append(obj2.Properties, Property{Type: TypeObject, Object: obj1})
	obj2.Properties = append(obj2.Properties, Property{Type: typeBoolean, Number: 1})

	// Retrieve nested object
	obj, err := obj2.ObjectProperty("", 8)
	if err != err {
		t.Errorf("Failed to retrieve object")
	}
	if len(obj.Properties) < 1 {
		t.Errorf("Properties missing for nested object")
	}
	if obj.Properties[0].Type != typeBoolean {
		t.Errorf("Wrong property type for nested object")
	}

	// Encode it.
	enc = buf[:]
	enc, err = Encode(&obj2, enc)
	if err != nil {
		t.Errorf("Encode of object failed")
	}

	// Decode it.
	dec = buf[1:]
	var dobj2 Object
	_, err = Decode(&dobj2, dec, false)
	if err != nil {
		t.Errorf("Decode of object failed with error: %v", err)
	}

	// Find some properties that exist.
	s, err := obj2.StringProperty("", 2)
	if err != nil {
		t.Errorf("StringProperty failed")
	}
	if s != "foo" {
		t.Errorf("StringProperty returned wrong String")
	}
	n, err := obj2.NumberProperty("", 3)
	if err != nil {
		t.Errorf("NumberProperty failed")
	}
	if n != 1 {
		t.Errorf("NumberProperty returned wrong Number")
	}
	obj, err = obj2.ObjectProperty("", 8)
	if err != nil {
		t.Errorf("ObjectProperty failed")
		return
	}
	if obj.Properties[0].Type != typeBoolean {
		t.Errorf("ObjectProperty returned object with wrong property")
	}
	prop, err := obj2.Property("", 9, typeBoolean)
	if err != nil {
		t.Errorf("Property failed")
		return
	}
	if prop.Number != 1 {
		t.Errorf("Property returned wrong Property")
	}

	// Try to find one that doesn't exist.
	prop, err = obj2.Property("", 10, TypeObject)
	if err != ErrPropertyNotFound {
		t.Errorf("Property(10) failed")
	}
}
