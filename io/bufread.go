package io

import (
	"io"
)

// Type bufReader enables reading a structure whose total length is not known
// upfront, via successive calls to ExtendTo whose arguments may depend on the
// state of the buffer after previous calls.
type bufReader struct {
	buf []byte
}

// Data provides a view into the data read so far. If the last call to ExtendTo
// returned io.EOF, the partially written data will appear in this view.
func (br *bufReader) Data() []byte {
	return br.buf
}

// ExtendTo reads into the internal buffer until it's at least the given size
// or until EOF is hit. If error is nil, len(br.Data(0) >= newSize. Otherwise,
// error may be io.EOF if not enough bytes could be read (including if zero
// bytes were read), or may be any other error in case of reader failure.
func (br *bufReader) ExtendTo(r io.Reader, newSize int) error {
	if newSize <= len(br.buf) {
		return nil
	}
	newBuf := make([]byte, newSize)
	n, err := io.ReadFull(r, newBuf[len(br.buf):])
	if err == nil {
		copy(newBuf, br.buf)
		br.buf = newBuf
		return nil
	}
	if err == io.EOF {
		return io.EOF
	}
	if err == io.ErrUnexpectedEOF {
		// In principle, this could be quadratically slow if we keep
		// reading 1 byte at a time. One fix: track "bufs [][]byte"
		// instead, and only flatten when ExtendTo succeeds.
		copy(newBuf, br.buf)
		br.buf = newBuf[:len(br.buf)+n]
		return io.EOF
	}
	return err
}
