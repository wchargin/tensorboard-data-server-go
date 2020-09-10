package io

import (
	"bytes"
	"io"
	"testing"
)

func TestScriptedReaderRead(t *testing.T) {
	sr := scriptedReader([]*bytes.Buffer{
		bytes.NewBuffer([]byte{0, 1, 2, 3}),
		bytes.NewBuffer([]byte{}),
		bytes.NewBuffer([]byte{4, 5, 6, 7, 8, 9}),
		bytes.NewBuffer([]byte{10}),
	})

	// Try to ReadFull three bytes at a time, and make sure that we get the
	// desired sequence.
	steps := []struct {
		buf []byte
		n   int
		err error
	}{
		// Read first buffer, with underread.
		{[]byte{0, 1, 2}, 3, nil},
		{[]byte{3, 0, 0}, 1, nil},
		{[]byte{0, 0, 0}, 0, io.EOF},
		// Read second buffer, which is empty.
		{[]byte{0, 0, 0}, 0, io.EOF},
		// Read third buffer, exactly.
		{[]byte{4, 5, 6}, 3, nil},
		{[]byte{7, 8, 9}, 3, nil},
		{[]byte{0, 0, 0}, 0, io.EOF},
		// Read fourth buffer, with underread.
		{[]byte{10, 0, 0}, 1, nil},
		{[]byte{0, 0, 0}, 0, io.EOF},
		// Read past end of buffer list.
		{[]byte{0, 0, 0}, 0, io.EOF},
		{[]byte{0, 0, 0}, 0, io.EOF},
	}
	for i, want := range steps {
		gotBuf := make([]byte, 3)
		gotN, gotErr := sr.Read(gotBuf)
		if gotN != want.n || gotErr != want.err || !bytes.Equal(gotBuf, want.buf) {
			t.Fatalf(
				"step %v: got %v, %v, %v; want %v, %v, %v",
				i+1,
				gotBuf, gotN, gotErr,
				want.buf, want.n, want.err,
			)
		}
	}
}

func TestScriptedReaderReadFull(t *testing.T) {
	sr := scriptedReader([]*bytes.Buffer{
		bytes.NewBuffer([]byte{0, 1, 2, 3}),
		bytes.NewBuffer([]byte{}),
		bytes.NewBuffer([]byte{4, 5, 6, 7, 8, 9}),
		bytes.NewBuffer([]byte{10}),
	})

	// Try to ReadFull three bytes at a time, and make sure that we get the
	// desired sequence.
	steps := []struct {
		buf []byte
		n   int
		err error
	}{
		// Read first buffer, with underread.
		{[]byte{0, 1, 2}, 3, nil},
		{[]byte{3, 0, 0}, 1, io.ErrUnexpectedEOF},
		// Read second buffer, which is empty.
		{[]byte{0, 0, 0}, 0, io.EOF},
		// Read third buffer, exactly.
		{[]byte{4, 5, 6}, 3, nil},
		{[]byte{7, 8, 9}, 3, nil},
		{[]byte{0, 0, 0}, 0, io.EOF},
		// Read fourth buffer, with underread.
		{[]byte{10, 0, 0}, 1, io.ErrUnexpectedEOF},
		// Read past end of buffer list.
		{[]byte{0, 0, 0}, 0, io.EOF},
		{[]byte{0, 0, 0}, 0, io.EOF},
	}
	for i, want := range steps {
		gotBuf := make([]byte, 3)
		gotN, gotErr := io.ReadFull(&sr, gotBuf)
		if gotN != want.n || gotErr != want.err || !bytes.Equal(gotBuf, want.buf) {
			t.Fatalf(
				"step %v: got %v, %v, %v; want %v, %v, %v",
				i+1,
				gotBuf, gotN, gotErr,
				want.buf, want.n, want.err,
			)
		}
	}
}
