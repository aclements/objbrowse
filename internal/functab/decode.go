// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package functab

import "encoding/binary"

type decoder struct {
	order   binary.ByteOrder
	ptrSize int
	data    []byte
	pos     uint64
}

func (d *decoder) Bytes(n uint64) []byte {
	v := d.data[d.pos : d.pos+n]
	d.pos += n
	return v
}

func (d *decoder) Uint8() uint8 {
	v := d.data[d.pos]
	d.pos++
	return v
}

func (d *decoder) Int8() int8 {
	return int8(d.Uint8())
}

func (d *decoder) Uint16() uint16 {
	v := d.order.Uint16(d.data[d.pos:])
	d.pos += 2
	return v
}

func (d *decoder) Int16() int16 {
	return int16(d.Uint16())
}

func (d *decoder) Uint32() uint32 {
	v := d.order.Uint32(d.data[d.pos:])
	d.pos += 4
	return v
}

func (d *decoder) Int32() int32 {
	return int32(d.Uint32())
}

func (d *decoder) Ptr() uint64 {
	var v uint64
	switch d.ptrSize {
	case 4:
		v = uint64(d.order.Uint32(d.data[d.pos:]))
	case 8:
		v = d.order.Uint64(d.data[d.pos:])
	default:
		panic("bad ptrSize")
	}
	d.pos += uint64(d.ptrSize)
	return v
}

func (d *decoder) CString() string {
	start := d.pos
	for d.data[d.pos] != 0 {
		d.pos++
	}
	d.pos++
	return string(d.data[start : d.pos-1])
}

func (d *decoder) Varint() int64 {
	val, read := binary.Varint(d.data[d.pos:])
	d.pos += uint64(read)
	return val
}

func (d *decoder) Uvarint() uint64 {
	val, read := binary.Uvarint(d.data[d.pos:])
	d.pos += uint64(read)
	return val
}
