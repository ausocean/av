/*
NAME
  meta.go

DESCRIPTION
  Package meta provides functions for adding to, modifying and reading
  metadata, as well as encoding and decoding functions.

AUTHOR
  Saxon Nelson-Milton <saxon@ausocean.org>

LICENSE
  Copyright (C) 2024 the Australian Ocean Lab (AusOcean). All Rights Reserved. 

  The Software and all intellectual property rights associated
  therewith, including but not limited to copyrights, trademarks,
  patents, and trade secrets, are and will remain the exclusive
  property of the Australian Ocean Lab (AusOcean).
*/


// Package meta provides functions for adding to, modifying and reading
// metadata, as well as encoding and decoding functions.
package meta

import (
	"encoding/binary"
	"errors"
	"fmt"
	"strings"
	"sync"
)

// This is the headsize of our metadata string,
// which is encoded int the data body of a pmt descriptor.
const headSize = 4

const (
	majVer = 1
	minVer = 0
)

// Indices of bytes for uint16 metadata length.
const (
	dataLenIdx = 2
)

var (
	errKeyAbsent            = errors.New("Key does not exist in map")
	errInvalidMeta          = errors.New("Invalid metadata given")
	ErrUnexpectedMetaFormat = errors.New("Unexpected meta format")
)

// Data provides functionality for the storage and encoding of metadata
// using a map.
type Data struct {
	mu    sync.RWMutex
	data  map[string]string
	order []string
	enc   []byte
}

// New returns a pointer to a new Metadata.
func New() *Data {
	return &Data{
		data: make(map[string]string),
		enc: []byte{
			0x00,                   // Reserved byte
			(majVer << 4) | minVer, // MS and LS versions
			0x00,                   // Data len byte1
			0x00,                   // Data len byte2
		},
	}
}

// NewWith creates a meta.Data and fills map with initial data given. If there
// is repeated key, then the latter overwrites the prior.
func NewWith(data [][2]string) *Data {
	m := New()
	m.order = make([]string, 0, len(data))
	for _, d := range data {
		if _, exists := m.data[d[0]]; !exists {
			m.order = append(m.order, d[0])
		}
		m.data[d[0]] = d[1]
	}
	return m
}

// NewFromMap creates a meta.Data from a map.
func NewFromMap(data map[string]string) *Data {
	m := New()
	m.order = make([]string, 0, len(data))
	for k, v := range data {
		m.data[k] = v
		m.order = append(m.order, k)
	}
	return m
}

// Add adds metadata with key and val.
func (m *Data) Add(key, val string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.data[key] = val
	for _, k := range m.order {
		if k == key {
			return
		}
	}
	m.order = append(m.order, key)
	return
}

// All returns the a copy of the map containing the meta data.
func (m *Data) All() map[string]string {
	m.mu.Lock()
	cpy := make(map[string]string)
	for k, v := range m.data {
		cpy[k] = v
	}
	m.mu.Unlock()
	return cpy
}

// Get returns the meta data for the passed key.
func (m *Data) Get(key string) (val string, ok bool) {
	m.mu.Lock()
	val, ok = m.data[key]
	m.mu.Unlock()
	return
}

// Delete deletes a meta entry in the map and returns error if it doesn’t exist.
func (m *Data) Delete(key string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.data[key]; ok {
		delete(m.data, key)
		for i, k := range m.order {
			if k == key {
				copy(m.order[i:], m.order[i+1:])
				m.order = m.order[:len(m.order)-1]
				break
			}
		}
		return
	}
	return
}

// Encode takes the meta data map and encodes into a byte slice with header
// describing the version, length of data and data in TSV format.
func (m *Data) Encode() []byte {
	if m.enc == nil {
		panic("Meta has not been initialized yet")
	}
	m.enc = m.enc[:headSize]

	// Iterate over map and append entries, only adding tab if we're not on the
	// last entry.
	var entry string
	for i, k := range m.order {
		v := m.data[k]
		entry += k + "=" + v
		if i+1 < len(m.data) {
			entry += "\t"
		}
	}
	m.enc = append(m.enc, []byte(entry)...)

	// Calculate and set data length in encoded meta header.
	dataLen := len(m.enc[headSize:])
	binary.BigEndian.PutUint16(m.enc[dataLenIdx:dataLenIdx+2], uint16(dataLen))
	return m.enc
}

// EncodeAsString takes the meta data map and encodes into a string with the data in
// TSV format. Unlike encode, the header with version and length of data is not
// included. This method is used for storing metadata in the cloud store.
func (m *Data) EncodeAsString() string {
	// Iterate over map and append entries, only adding tab if we're not on the
	// last entry.
	var str string
	for i, k := range m.order {
		v := m.data[k]
		str += k + "=" + v
		if i+1 < len(m.data) {
			str += "\t"
		}
	}
	return str
}

// Keys returns all keys in a slice of metadata d.
func Keys(d []byte) ([]string, error) {
	m, err := GetAll(d)
	if err != nil {
		return nil, err
	}
	k := make([]string, len(m))
	for i, kv := range m {
		k[i] = kv[0]
	}
	return k, nil
}

// Get returns the value for the given key in d.
func Get(key string, d []byte) (string, error) {
	err := checkMeta(d)
	if err != nil {
		return "", err
	}
	d = d[headSize:]
	entries := strings.Split(string(d), "\t")
	for _, entry := range entries {
		kv := strings.Split(entry, "=")
		if kv[0] == key {
			return kv[1], nil
		}
	}
	return "", errKeyAbsent
}

// GetAll returns metadata keys and values from d.
func GetAll(d []byte) ([][2]string, error) {
	err := checkMeta(d)
	if err != nil {
		return nil, err
	}
	d = d[headSize:]
	entries := strings.Split(string(d), "\t")
	all := make([][2]string, len(entries))
	for i, entry := range entries {
		kv := strings.Split(entry, "=")
		if len(kv) != 2 {
			return nil, ErrUnexpectedMetaFormat
		}
		copy(all[i][:], kv)
	}
	return all, nil
}

// GetAllAsMap returns a map containing keys and values from a slice d containing
// metadata.
func GetAllAsMap(d []byte) (map[string]string, error) {
	err := checkMeta(d)
	if err != nil {
		return nil, err
	}

	// Skip the header, which is our data length and version.
	d = d[headSize:]

	return GetAllFromString(string(d))
}

// GetAllFromString returns a map containing keys and values from a string s containing
// metadata.
func GetAllFromString(s string) (map[string]string, error) {
	// Each metadata entry (key and value) is seperated by a tab, so split at tabs
	// to get individual entries.
	entries := strings.Split(s, "\t")

	// Go through entries and add to all map.
	all := make(map[string]string)
	for _, entry := range entries {
		// Keys and values are seperated by '=', so split and check that len(kv)=2.
		kv := strings.Split(entry, "=")
		if len(kv) != 2 {
			return nil, fmt.Errorf("not enough key-value pairs (kv: %v)", kv)
		}
		all[kv[0]] = kv[1]
	}
	return all, nil
}

// checkHeader checks that a valid metadata header exists in the given data.
func checkMeta(d []byte) error {
	if len(d) == 0 || d[0] != 0 || binary.BigEndian.Uint16(d[2:headSize]) != uint16(len(d[headSize:])) {
		return errInvalidMeta
	}
	return nil
}
