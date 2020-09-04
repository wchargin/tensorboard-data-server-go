package io

import (
	"hash/crc32"
)

const (
	crcMaskDelta uint32 = 0xa282ead8
)

var castagnoliTable = crc32.MakeTable(crc32.Castagnoli)

func maskCRC(crc uint32) uint32 {
	return ((crc >> 15) | (crc << 17)) + crcMaskDelta
}

// computeMaskedCRC computes a Castagnoli CRC-32 checksum and applies a masking
// permutation to it. This is the checksum operation used by TFRecords.
func computeMaskedCRC(data []byte) uint32 {
	return maskCRC(crc32.Checksum(data, castagnoliTable))
}
