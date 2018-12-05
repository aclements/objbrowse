// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package functab

import (
	"strconv"
)

type FuncData struct {
	fi  *fileInfo
	ptr uint64
}

func (f FuncData) Read(size uint64) ([]byte, error) {
	return f.fi.mmap.Data(f.ptr, size)
}

func (f FuncData) StackMap() ([]Bitmap, error) {
	// Read the header
	hdr, err := f.Read(8)
	if err != nil {
		return nil, err
	}
	d := decoder{f.fi.order, f.fi.ptrSize, hdr, 0}
	n := d.Uint32()
	nbit := d.Uint32()
	bytes := (nbit + 7) / 8

	// Now we know the size, so the read the whole thing.
	data, err := f.Read(8 + uint64(n*bytes))
	if err != nil {
		return nil, err
	}

	// Read the bitmaps
	d.data = data
	bitmaps := make([]Bitmap, n)
	for i := range bitmaps {
		bitmaps[i].N = int(nbit)
		bitmaps[i].Bytes = d.Bytes(uint64(bytes))
	}
	return bitmaps, nil
}

type Bitmap struct {
	N     int // number of bits
	Bytes []byte
}

func (b Bitmap) Bit(i int) bool {
	if i < 0 || i >= b.N {
		panic("index out of bounds")
	}
	return (b.Bytes[i/8]>>uint(i%8))&1 != 0
}

func (b Bitmap) String() string {
	s := make([]byte, b.N)
	for i := range s {
		if b.Bit(i) {
			s[i] = '1'
		} else {
			s[i] = '0'
		}
	}
	return string(s)
}

// Hex returns a hexadecimal representation of b in the form
// "bits:hex". Bits is a decimal number of bits in the bitmap, and hex
// is the bitmap written as base 16 octets, where the bit 0 is the
// least-significant bit in the first octet.
func (b Bitmap) Hex() string {
	const digits = "0123456789abcdef"
	buf := make([]byte, 0, 4+(b.N+3)/4)
	buf = strconv.AppendInt(buf, int64(b.N), 10)
	buf = append(buf, ':')
	for i := 0; i < (b.N+7)/8; i++ {
		byte := b.Bytes[i]
		buf = append(buf, digits[byte>>4])
		buf = append(buf, digits[byte&0xF])
	}
	return string(buf)
}
