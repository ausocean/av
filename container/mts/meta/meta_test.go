/*
NAME
  meta_test.go

AUTHOR
  Saxon Nelson-Milton <saxon@ausocean.org>

LICENSE
  Copyright (C) 2024 the Australian Ocean Lab (AusOcean). All Rights Reserved. 

  The Software and all intellectual property rights associated
  therewith, including but not limited to copyrights, trademarks,
  patents, and trade secrets, are and will remain the exclusive
  property of the Australian Ocean Lab (AusOcean).
*/


package meta

import (
	"bytes"
	"encoding/binary"
	"reflect"
	"testing"
)

const (
	TimestampKey = "ts"
	LocationKey  = "loc"
	tstKey1      = LocationKey
	tstData1     = "a,b,c"
	tstKey2      = TimestampKey
	tstData2     = "12345678"
	tstData3     = "d,e,f"
)

// TestAddAndGet ensures that we can add metadata and then successfully get it.
func TestAddAndGet(t *testing.T) {
	meta := New()
	meta.Add(tstKey1, tstData1)
	meta.Add(tstKey2, tstData2)
	if data, ok := meta.Get(tstKey1); !ok {
		t.Errorf("Could not get data for key: %v\n", tstKey1)
		if data != tstData1 {
			t.Error("Did not get expected data")
		}
	}

	if data, ok := meta.Get(tstKey2); !ok {
		t.Errorf("Could not get data for key: %v", tstKey2)
		if data != tstData2 {
			t.Error("Did not get expected data")
		}
	}
}

// TestUpdate checks that we can use Meta.Add to actually update metadata
// if it already exists in the Meta map.
func TestUpdate(t *testing.T) {
	meta := New()
	meta.Add(tstKey1, tstData1)
	meta.Add(tstKey1, tstData3)

	if data, ok := meta.Get(tstKey1); !ok {
		t.Errorf("Could not get data for key: %v\n", tstKey1)
		if data != tstData2 {
			t.Errorf("Data did not correctly update for key: %v\n", tstKey1)
		}
	}
}

// TestAll ensures we can get a correct map using Meta.All() after adding some data
func TestAll(t *testing.T) {
	meta := New()
	tstMap := map[string]string{
		tstKey1: tstData1,
		tstKey2: tstData2,
	}

	meta.Add(tstKey1, tstData1)
	meta.Add(tstKey2, tstData2)
	metaMap := meta.All()

	if !reflect.DeepEqual(metaMap, tstMap) {
		t.Errorf("Map not correct. Got: %v, want: %v", metaMap, tstMap)
	}
}

// TestGetAbsentKey ensures that we get the expected error when we try to get with
// key that does not yet exist in the Meta map.
func TestGetAbsentKey(t *testing.T) {
	meta := New()

	if _, ok := meta.Get(tstKey1); ok {
		t.Error("Get for absent key incorrectly returned'ok'")
	}
}

// TestDelete ensures we can remove a data entry in the Meta map.
func TestDelete(t *testing.T) {
	meta := New()
	meta.Add(tstKey1, tstData1)
	meta.Delete(tstKey1)
	if _, ok := meta.Get(tstKey1); ok {
		t.Error("Get incorrectly returned okay for absent key")
	}
}

// TestEncode checks that we're getting the correct byte slice from Meta.Encode().
func TestEncode(t *testing.T) {
	meta := New()
	meta.Add(tstKey1, tstData1)
	meta.Add(tstKey2, tstData2)

	dataLen := len(tstKey1+tstData1+tstKey2+tstData2) + 3
	header := [4]byte{
		0x00,
		0x10,
	}
	binary.BigEndian.PutUint16(header[2:4], uint16(dataLen))
	expectedOut := append(header[:], []byte(
		tstKey1+"="+tstData1+"\t"+
			tstKey2+"="+tstData2)...)

	got := meta.Encode()
	if !bytes.Equal(expectedOut, got) {
		t.Errorf("Did not get expected out. \nGot : %v, \nwant: %v\n", got, expectedOut)
	}
}

// TestGetFrom checks that we can correctly obtain a value for a partiular key
// from a string of metadata using the ReadFrom func.
func TestGetFrom(t *testing.T) {
	tstMeta := append([]byte{0x00, 0x10, 0x00, 0x12}, "loc=a,b,c\tts=12345"...)

	tests := []struct {
		key  string
		want string
	}{
		{
			LocationKey,
			"a,b,c",
		},
		{
			TimestampKey,
			"12345",
		},
	}

	for _, test := range tests {
		got, err := Get(test.key, []byte(tstMeta))
		if err != nil {
			t.Errorf("Unexpected err: %v\n", err)
		}
		if got != test.want {
			t.Errorf("Did not get expected out. \nGot : %v, \nwant: %v\n", got, test.want)
		}
	}
}

// TestGetAll checks that meta.GetAll can correctly get all metadata
// from descriptor data.
func TestGetAll(t *testing.T) {
	tstMeta := append([]byte{0x00, 0x10, 0x00, 0x12}, "loc=a,b,c\tts=12345"...)
	want := [][2]string{
		{
			LocationKey,
			"a,b,c",
		},
		{
			TimestampKey,
			"12345",
		},
	}
	got, err := GetAll(tstMeta)
	if err != nil {
		t.Errorf("Unexpected error: %v\n", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Did not get expected out. \nGot : %v, \nWant: %v\n", got, want)
	}
}

// TestGetAllAsMap checks that GetAllAsMap will correctly return a map of meta
// keys and values from a slice of meta.
func TestGetAllAsMap(t *testing.T) {
	tstMeta := append([]byte{0x00, 0x10, 0x00, 0x12}, "loc=a,b,c\tts=12345"...)
	want := map[string]string{
		LocationKey:  "a,b,c",
		TimestampKey: "12345",
	}
	got, err := GetAllAsMap(tstMeta)
	if err != nil {
		t.Errorf("Unexpected error: %v\n", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Did not get expected out. \nGot : %v, \nWant: %v\n", got, want)
	}
}

// TestKeys checks that we can successfully get keys from some metadata using
// the meta.Keys method.
func TestKeys(t *testing.T) {
	tstMeta := append([]byte{0x00, 0x10, 0x00, 0x12}, "loc=a,b,c\tts=12345"...)
	want := []string{LocationKey, TimestampKey}
	got, err := Keys(tstMeta)
	if err != nil {
		t.Errorf("Unexpected error: %v\n", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Did not get expected out. \nGot : %v, \nWant: %v\n", got, want)
	}
}

// TestNewFromMap checks that we can successfully create a new data struct from a map.
func TestNewFromMap(t *testing.T) {
	want := map[string]string{
		LocationKey:  "a,b,c",
		TimestampKey: "12345",
	}

	meta := NewFromMap(want)

	got := meta.All()

	if !reflect.DeepEqual(got, want) {
		t.Errorf("Did not get expected out. \nGot : %v, \nWant: %v\n", got, want)
	}
}

// TestEncodeAsString checks that metadata is correctly encoded as a string.
func TestEncodeAsString(t *testing.T) {
	meta := NewFromMap(map[string]string{
		LocationKey:  "a,b,c",
		TimestampKey: "12345",
	})

	got := meta.EncodeAsString()
	want1 := "loc=a,b,c\tts=12345"
	want2 := "ts=12345\tloc=a,b,c"

	if got != want1 && got != want2 {
		t.Errorf("Did not get expected out. \nGot : %v, \nWant: %v or %v\n", got, want1, want2)
	}
}

// TestDeleteOrder checks that the order of keys is correct after a deletion.
func TestDeleteOrder(t *testing.T) {
	tests := []struct {
		delKey string
		want   []string
	}{
		{
			"key1",
			[]string{"key2", "key3", "key4"},
		},
		{
			"key2",
			[]string{"key1", "key3", "key4"},
		},
		{
			"key3",
			[]string{"key1", "key2", "key4"},
		},
		{
			"key4",
			[]string{"key1", "key2", "key3"},
		},
	}

	for _, test := range tests {
		t.Logf("deleting %s", test.delKey)
		meta := NewWith([][2]string{
			{"key1", "val1"},
			{"key2", "val2"},
			{"key3", "val3"},
			{"key4", "val4"},
		})
		meta.Delete(test.delKey)

		got := meta.order
		want := test.want
		if !reflect.DeepEqual(got, want) {
			t.Errorf("Did not get expected out. \nGot:  %v, \nWant: %v\n", got, want)
		}
	}
}
