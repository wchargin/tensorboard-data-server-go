package io

import (
	"bytes"
	"errors"
	"io"
	"testing"
)

func TestSingleRead(t *testing.T) {
	var br bufReader
	r := bytes.NewBuffer([]byte{0, 1, 1, 2, 3, 5})
	if err := br.ExtendTo(r, 4); err != nil {
		t.Errorf("bad read: got %v, want nil", err)
	}
	if got, want := br.Data(), []byte{0, 1, 1, 2}; !bytes.Equal(got, want) {
		t.Errorf("bad contents: got %v, want %v", got, want)
	}
	if got, want := r.Bytes(), []byte{3, 5}; !bytes.Equal(got, want) {
		t.Errorf("bad stream remainder: got %v, want %v", got, want)
	}
}

func TestMultiReadSuccess(t *testing.T) {
	var br bufReader
	r := bytes.NewBuffer([]byte{0, 1, 1, 2})
	if err := br.ExtendTo(r, 5); err != io.EOF {
		t.Errorf("bad error #1: got %v, want %v", err, io.EOF)
	}
	r.Write([]byte{3, 5, 8, 13}) // Buffer.Write error always nil
	if err := br.ExtendTo(r, 5); err != nil {
		t.Errorf("bad error #2: got %v, want %v", err, nil)
	}
	if got, want := br.Data(), []byte{0, 1, 1, 2, 3}; !bytes.Equal(got, want) {
		t.Errorf("bad contents: got %v, want %v", got, want)
	}
	if got, want := r.Bytes(), []byte{5, 8, 13}; !bytes.Equal(got, want) {
		t.Errorf("bad stream remainder: got %v, want %v", got, want)
	}
}

func TestMultiReadEOF(t *testing.T) {
	var br bufReader
	r := bytes.NewBuffer([]byte{0, 1, 1, 2})
	if err := br.ExtendTo(r, 5); err != io.EOF {
		t.Errorf("bad error #1: got %v, want %v", err, io.EOF)
	}
	if err := br.ExtendTo(r, 5); err != io.EOF {
		t.Errorf("bad error #2: got %v, want %v", err, io.EOF)
	}
	if got, want := r.Bytes(), []byte{}; !bytes.Equal(got, want) {
		t.Errorf("bad stream remainder: got %v, want %v", got, want)
	}
}

func TestTrivialRead(t *testing.T) {
	var br bufReader
	r := bytes.NewBuffer([]byte{0, 1, 1, 2, 3, 5})
	br.ExtendTo(r, 5)
	if err := br.ExtendTo(r, 3); err != nil {
		t.Errorf("bad error: got %v, want %v", err, nil)
	}
	if got, want := br.Data(), []byte{0, 1, 1, 2, 3}; !bytes.Equal(got, want) {
		t.Errorf("bad contents: got %v, want %v", got, want)
	}
	if got, want := r.Bytes(), []byte{5}; !bytes.Equal(got, want) {
		t.Errorf("bad stream remainder: got %v, want %v", got, want)
	}
}

type failureReader struct {
	e error
}

func (r failureReader) Read([]byte) (int, error) {
	return 0, r.e
}

func TestFailedRead(t *testing.T) {
	var br bufReader
	want := errors.New("not so fast")
	got := br.ExtendTo(failureReader{want}, 7)
	if got != want {
		t.Errorf("bad error: got %v, want %v", got, want)
	}
}

func TestZeroRead(t *testing.T) {
	var br bufReader
	r := failureReader{errors.New("shouldn't see this")}
	if err := br.ExtendTo(r, 0); err != nil {
		t.Errorf("bad error: got %v, want %v", err, nil)
	}
	if len(br.Data()) != 0 {
		t.Errorf("bad data: got %v, wanted empty", br.Data())
	}
}
