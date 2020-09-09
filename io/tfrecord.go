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
	// TFRecord header: little-endian u64 length, u32 length-CRC. Only the
	// prefix of length headerRead is valid.
	header [headerLength]byte
	// Number of bytes of header that have actually been read.
	headerRead int
	// Everything past the header in the TFRecord: the data buffer, plus a
	// little-endian u32 CRC of the data buffer. Only the prefix of length
	// dataPlusFooterRead is valid.
	dataPlusFooter []byte
	// Number of bytes of dataPlusFooter that have actually been read.
	dataPlusFooterRead int
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
	state := *statePtr

	if state.headerRead < headerLength {
		dst := state.header[state.headerRead:]
		if err := readRemaining(r, dst, &state.headerRead); err != nil {
			return nil, err
		}

		lengthBuf := state.header[:lengthCRCOffset]
		lengthCRCBuf := state.header[lengthCRCOffset:]
		lengthCRC := binary.LittleEndian.Uint32(lengthCRCBuf)
		if actualCRC := computeMaskedCRC(lengthBuf); actualCRC != lengthCRC {
			return nil, status.Errorf(codes.DataLoss, "length CRC mismatch; cannot read rest of file: got %#x, want %#x", actualCRC, lengthCRC)
		}
		length := binary.LittleEndian.Uint64(lengthBuf)

		dataPlusFooterLengthUint64 := length + uint64(footerLength)
		dataPlusFooterLength := int(dataPlusFooterLengthUint64)
		if uint64(dataPlusFooterLength) != dataPlusFooterLengthUint64 {
			return nil, status.Errorf(codes.OutOfRange, "record too large for system: %v", dataPlusFooterLengthUint64)
		}
		state.dataPlusFooter = make([]byte, dataPlusFooterLength)
	}

	if state.dataPlusFooterRead < len(state.dataPlusFooter) {
		dst := state.dataPlusFooter
		if state.dataPlusFooterRead > 0 {
			dst = dst[state.dataPlusFooterRead:]
		}
		if err := readRemaining(r, dst, &state.dataPlusFooterRead); err != nil {
			return nil, err
		}
	}

	dataLength := len(state.dataPlusFooter) - footerLength
	dataBuf := state.dataPlusFooter[:dataLength]
	dataCRC := binary.LittleEndian.Uint32(state.dataPlusFooter[dataLength:])
	result := TFRecord{
		Data:      dataBuf,
		maskedCRC: dataCRC,
	}
	return &result, nil
}

func readRemaining(r io.Reader, buf []byte, readPtr *int) error {
	n, err := io.ReadFull(r, buf)
	*readPtr += n
	if err == io.EOF || err == io.ErrUnexpectedEOF {
		return io.EOF
	}
	if err != nil {
		return err
	}
	return nil
}
