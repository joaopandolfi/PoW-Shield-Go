package utils

import (
	"encoding/binary"
	"time"
)

func WriteTimestamp(buffer []byte, ts uint64, off int) int {
	high := uint32(ts >> 32)
	low := uint32(ts & 0xffffffff)

	binary.BigEndian.PutUint32(buffer[off:], high)
	binary.BigEndian.PutUint32(buffer[off+4:], low)

	return off + 8
}

func ReadTimestamp(buffer []byte, off int) uint64 {
	high := binary.BigEndian.Uint32(buffer[off:])
	low := binary.BigEndian.Uint32(buffer[off+4:])

	return uint64(high)*0x100000000 + uint64(low)
}

func Now() uint64 {
	return uint64(time.Now().UnixNano())
}

func AbsDiff(a, b uint64) uint64 {
	if a > b {
		return a - b
	}

	return b - a
}
