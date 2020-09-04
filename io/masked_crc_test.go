package io

import (
	"testing"
)

func TestComputeMaskedCRC(t *testing.T) {
	// Checksum extracted from real TensorFlow event file with record:
	// tf.compat.v1.Event(file_version=b"CRC test, one two")
	data := []byte("\x1a\x11CRC test, one two")
	want := uint32(0x5794d08a)
	got := computeMaskedCRC(data)
	if got != want {
		t.Errorf("bad CRC: got %#x, want %#x", got, want)
	}
}
