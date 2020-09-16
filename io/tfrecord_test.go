package io

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestNewWriteReadRoundtrip(t *testing.T) {
	data := []byte("\x1a\x0dbrain.Event:2")
	input := NewTFRecord(data)
	if err := input.Checksum(); err != nil {
		t.Errorf("input.Checksum(): got %v, want nil", err)
	}
	var buf bytes.Buffer
	if err := input.Write(&buf); err != nil {
		t.Fatalf("input.Write(&buf): got %v, want nil", err)
	}
	if bs := input.ByteSize(); bs != buf.Len() {
		t.Errorf("predicted ByteSize != actual length: got %v, want %v", bs, buf.Len())
	}
	var state *TFRecordState
	output, err := ReadRecord(&state, &buf)
	if err != nil {
		t.Fatalf("ReadRecord(&state, &buf): %v", err)
	}
	if !bytes.Equal(output.Data, input.Data) {
		t.Errorf("round-trip: got %q, want %q", string(output.Data), string(input.Data))
	}
	if buf.Len() != 0 {
		t.Errorf("left-over buffer: got %q, want empty", buf.Bytes())
	}
}

func TestReadRecordSuccess(t *testing.T) {
	// Event file with `tf.summary.scalar("accuracy", 0.99, step=77)`
	// dumped via `xxd logs/*`.
	record1A := "\x09\x00\x00\x80\x38\x99"
	record1B := "\xd6\xd7\x41\x1a\x0dbrain.Event:2"
	record1 := record1A + record1B
	record2 := "" +
		"\x09\xc4\x05\xb7\x3d\x99\xd6\xd7\x41" +
		"\x10\x4d\x2a\x25" +
		"\x0a\x23\x0a\x08accuracy" +
		"\x42\x0a\x08\x01\x12\x00\x22\x04\xa4\x70\x7d\x3f\x4a" +
		"\x0b\x0a\x09\x0a\x07scalars" +
		""

	sr := scriptedReader([]*bytes.Buffer{
		// First 5 bytes of header
		bytes.NewBufferString("\x18\x00\x00\x00\x00"),
		// Next 6 bytes of header
		bytes.NewBufferString("\x00\x00\x00" + "\xa3\x7f\x4b"),
		// Last byte of header and first 6 bytes of content
		bytes.NewBufferString("\x22" + record1A),
		// Rest of content and footer, plus first 2 bytes of next record header
		bytes.NewBufferString(record1B + "\x12\x4b\x36\xab" + "\x32\x00"),
		// Entirety of next record
		bytes.NewBufferString("" +
			"\x00\x00\x00\x00\x00\x00" + "\x24\x19\x56\xec" +
			record2 +
			"\xa5\x5b\x64\x33" +
			""),
	})

	var st *TFRecordState

	steps := []struct {
		rec *string // nil if record shouldn't be ready yet
		err error
	}{
		// 5 bytes header
		{nil, io.EOF},
		// 6 bytes header
		{nil, io.EOF},
		// Full header, 6 bytes content
		{nil, io.EOF},
		// Rest of content and footer
		{&record1, nil},
		// 2 bytes header (same read as previous)
		{nil, io.EOF},
		// Full record
		{&record2, nil},
		// Nothing
		{nil, io.EOF},
		{nil, io.EOF},
	}
	for i, want := range steps {
		gotRec, gotErr := ReadRecord(&st, &sr)
		ok := true
		if gotErr != want.err {
			ok = false
		}
		if (gotRec == nil) != (want.rec == nil) {
			ok = false
		} else if gotRec != nil && string(gotRec.Data) != *want.rec {
			ok = false
		}
		if !ok {
			gotRecDisplay := "nil"
			if gotRec != nil {
				gotRecDisplay = fmt.Sprintf("%q, len=%v", string(gotRec.Data), len(gotRec.Data))
			}
			wantRecDisplay := "nil"
			if want.rec != nil {
				wantRecDisplay = fmt.Sprintf("%q, len=%v", *want.rec, len(*want.rec))
			}
			t.Errorf("step %v: got %v (%v), %v; want %v (%v), %v",
				i+1,
				gotRec, gotRecDisplay, gotErr,
				want.rec, wantRecDisplay, want.err)
		}
		if gotRec != nil {
			if err := gotRec.Checksum(); err != nil {
				t.Errorf("step %v: checksum failure: %v", i+1, err)
			}
			st = nil
		}
	}
}

func TestReadRecordLengthCRCMismatch(t *testing.T) {
	header := "\x18\x00\x00\x00\x00\x00\x00\x00" + "\x99\x7f\x4b\x55"
	body := "123456789abcdef012345678\x00\x00\x00\x00"
	sr := scriptedReader([]*bytes.Buffer{
		bytes.NewBufferString(header + body),
	})

	var st *TFRecordState
	rec, err := ReadRecord(&st, &sr)
	wantCode := codes.DataLoss
	wantMsg := "got 0x224b7fa3, want 0x554b7f99"
	if rec != nil || !grpcErrorLike(err, wantCode, wantMsg) {
		t.Errorf("got %v, %q; want nil, code=%v, msg~=%q", rec, err, wantCode, wantMsg)
	}
}

func TestReadRecordDataCRCMismatch(t *testing.T) {
	header := "\x18\x00\x00\x00\x00\x00\x00\x00" + "\xa3\x7f\x4b\x22"
	body := "123456789abcdef012345678\xce\x9b\x57\x13"
	sr := scriptedReader([]*bytes.Buffer{
		bytes.NewBufferString(header + body),
	})

	var st *TFRecordState
	rec, readErr := ReadRecord(&st, &sr)
	if rec == nil || readErr != nil {
		t.Fatalf("got %v, %v; want real record, nil", rec, readErr)
	}
	cksumErr := rec.Checksum()
	wantCode := codes.DataLoss
	wantMsg := "want 0x13579bce"
	if !grpcErrorLike(cksumErr, wantCode, wantMsg) {
		t.Errorf("got %v, %q; want nil, code=%v, msg~=%q", rec, cksumErr, wantCode, wantMsg)
	}
}

// grpcErrorLike checks whether err is a gRPC status with the provided code and
// a message that contains the provided string as a substring.
func grpcErrorLike(err error, wantCode codes.Code, wantMsgSubstr string) bool {
	status, ok := status.FromError(err)
	if !ok {
		return false
	}
	if status.Code() != wantCode {
		return false
	}
	msg := status.Message()
	if !strings.Contains(msg, wantMsgSubstr) {
		return false
	}
	return true
}
