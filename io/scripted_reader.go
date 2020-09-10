package io

import (
	"bytes"
	"io"
)

// Type scriptedReader implements io.Reader by draining one buffer at a time.
// Unlike io.MultiReader, scriptedReader emits an EOF after each buffer. This
// simulates reading a growing file, for testing purposes.
type scriptedReader []*bytes.Buffer

func (sr *scriptedReader) Read(p []byte) (n int, err error) {
	for len(*sr) > 0 {
		buf := (*sr)[0]
		n, err := buf.Read(p)
		if n == 0 && err == io.EOF {
			*sr = (*sr)[1:]
		}
		return n, err
	}
	return 0, io.EOF
}
