package io

import (
	"encoding/binary"
	"io"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// From TensorFlow `record_writer.cc` comments:
// Format of a single record:
//  uint64    length
//  uint32    masked crc of length
//  byte      data[length]
//  uint32    masked crc of data
const (
	lengthOffset    int = 0
	lengthCRCOffset int = 8
	dataOffset      int = lengthCRCOffset + 4
	headerLength    int = dataOffset
	footerLength    int = 4
)

type TFRecord struct {
	Data      []byte
	maskedCRC uint32
}

// Checksum validates the integrity of the record by computing its CRC-32 and
// checking it against the expected value. It returns an error if the CRCs do
// not match.
func (rec *TFRecord) Checksum() error {
	if actualMaskedCRC := computeMaskedCRC(rec.Data); actualMaskedCRC != rec.maskedCRC {
		return status.Errorf(codes.DataLoss, "data CRC mismatch: got %#x, want %#x", actualMaskedCRC, rec.maskedCRC)
	}
	return nil
}

type TFRecordState struct {
	buf bufReader
}

// ReadRecord attempts to read a TFRecord, behaving nicely in the face of
// truncations. If the record is truncated, the first return value is nil and
// the second is io.EOF, and the state buffer will be updated to contain the
// prefix of the raw record that was read. The same state buffer should be
// passed to a subsequent call to ReadRecord that it may continue where it left
// off. The state buffer must not be nil, but may point to nil, and indeed must
// point to nil on the first invocation for each record.
//
// The record's length field is always validated against its checksum, but the
// full data is only validated if you call Checksum() on the result.
func ReadRecord(statePtr **TFRecordState, r io.Reader) (*TFRecord, error) {
	if *statePtr == nil {
		*statePtr = new(TFRecordState)
	}
	var state *bufReader = &(*statePtr).buf

	if err := state.ExtendTo(r, headerLength); err != nil {
		return nil, err
	}
	lengthBuf := state.Data()[lengthOffset:lengthCRCOffset]
	lengthCRC := binary.LittleEndian.Uint32(state.Data()[lengthCRCOffset:dataOffset])
	if actualCRC := computeMaskedCRC(lengthBuf); actualCRC != lengthCRC {
		return nil, status.Errorf(codes.DataLoss, "length CRC mismatch; cannot read rest of file: got %#x, want %#x", actualCRC, lengthCRC)
	}
	length := binary.LittleEndian.Uint64(lengthBuf)

	totalLengthUint64 := uint64(headerLength) + length + uint64(footerLength)
	totalLength := int(totalLengthUint64)
	if uint64(totalLength) != totalLengthUint64 {
		return nil, status.Errorf(codes.OutOfRange, "record too large for system: %v", totalLengthUint64)
	}
	if err := state.ExtendTo(r, totalLength); err != nil {
		return nil, err
	}

	// Cast to int is safe because totalLength (which is greater) fits in
	// int, from above.
	endOfData := headerLength + int(length)

	dataBuf := state.Data()[headerLength:endOfData]
	dataCRC := binary.LittleEndian.Uint32(state.Data()[endOfData:])
	result := TFRecord{
		Data:      dataBuf,
		maskedCRC: dataCRC,
	}
	return &result, nil
}
