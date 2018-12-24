// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package functab

import (
	"bytes"
	"encoding/binary"
	"fmt"

	"github.com/aclements/objbrowse/internal/obj"
)

type FuncTab struct {
	Funcs []*Func
	EndPC uint64
}

type Func struct {
	PC       uint64
	Name     string
	PCSP     PCData
	PCData   []PCData
	FuncData []FuncData
}

type symtabHdr struct {
	Magic     uint32
	_         uint16
	PCQuantum uint8
	PtrSize   uint8
}

type fileInfo struct {
	mmap      obj.Mem
	order     binary.ByteOrder
	ptrSize   int
	pcQuantum uint8
}

// NewFuncTab decodes a Go function table from the contents of a
// "runtime.pclntab" symbol.
func NewFuncTab(data []byte, mmap obj.Mem) (*FuncTab, error) {
	var order binary.ByteOrder
	var hdr symtabHdr
	for _, order = range []binary.ByteOrder{binary.LittleEndian, binary.BigEndian} {
		if err := binary.Read(bytes.NewBuffer(data), order, &hdr); err != nil {
			return nil, err
		}
		if hdr.Magic == 0xfffffffb {
			goto hdrGood
		}
	}
	return nil, fmt.Errorf("bad magic word in header %#x", hdr.Magic)
hdrGood:

	d := decoder{order: order, ptrSize: int(hdr.PtrSize), data: data, pos: 8}
	fi := &fileInfo{mmap, d.order, d.ptrSize, hdr.PCQuantum}

	// Read func PC/offset table.
	//
	// See cmd/link/internal/ld/pcln.go:pclntab
	nfunc := d.Ptr()
	funcs := make([]*Func, nfunc)
	offsets := make([]uint64, nfunc)
	for i := range offsets {
		d.Ptr() // PC (will read from func later)
		offsets[i] = d.Ptr()
	}
	endPC := d.Ptr()
	d.Uint32() // fileTabOffset

	// Read func structures.
	for i := range funcs {
		d.pos = offsets[i]

		// Fixed struct.
		// See runtime/runtime2.go:_func
		pc := d.Ptr()
		nameoff := d.Int32()
		d.Int32()  // args
		d.Uint32() // deferreturn
		pcspOff := d.Uint32()
		pcsp := PCData{fi, pc, data[pcspOff:]}
		d.Uint32() // pcfile
		d.Uint32() // pcln
		npcdata := d.Uint32()
		d.Uint8()  // funcID
		d.Uint16() // unused
		nfuncdata := d.Uint8()

		// PC data offsets (npcdata * uint32)
		pcdata := make([]PCData, npcdata)
		for i := range pcdata {
			off := d.Uint32()
			pcdata[i] = PCData{fi, pc, data[off:]}
		}

		// Func data offsets (nfuncdata * ptr)
		if d.ptrSize == 8 && d.pos&4 != 0 {
			// Func data is ptr-aligned.
			d.pos += 4
		}
		funcdata := make([]FuncData, nfuncdata)
		for i := range funcdata {
			funcdata[i] = FuncData{fi, d.Ptr()}
		}

		// Get name.
		d.pos = uint64(nameoff)
		name := d.CString()

		fn := &Func{pc, name, pcsp, pcdata, funcdata}
		funcs[i] = fn
	}

	return &FuncTab{
		Funcs: funcs,
		EndPC: endPC,
	}, nil
}

// PCDATA and FUNCDATA indexes.
//
// From runtime/symtab.go.
const (
	_PCDATA_StackMapIndex       = 0
	_PCDATA_InlTreeIndex        = 1
	_PCDATA_RegMapIndex         = 2
	_FUNCDATA_ArgsPointerMaps   = 0
	_FUNCDATA_LocalsPointerMaps = 1
	_FUNCDATA_InlTree           = 2
	_FUNCDATA_RegPointerMaps    = 3
	_FUNCDATA_StackObjects      = 4
	_ArgsSizeUnknown            = -0x80000000
)

type Liveness struct {
	Index        PCTable
	Args, Locals []Bitmap
}

func (f Func) Liveness() (Liveness, error) {
	if len(f.PCData) <= _PCDATA_StackMapIndex {
		return Liveness{}, nil
	}

	// Fetch the pointer bitmaps.
	args, err := f.FuncData[_FUNCDATA_ArgsPointerMaps].StackMap()
	if err != nil {
		return Liveness{}, nil
	}
	locals, err := f.FuncData[_FUNCDATA_LocalsPointerMaps].StackMap()
	if err != nil {
		return Liveness{}, nil
	}

	// Fetch the stack map index.
	stackMap := f.PCData[_PCDATA_StackMapIndex].Decode()

	// Apply runtime conventions to the stack map.
	if len(stackMap.PCs) > 0 {
		// Index -1 actually means index 0.
		for i, v := range stackMap.Values {
			if v == -1 {
				stackMap.Values[i] = 0
			}
		}

		// If there's any stack map, then the runtime uses
		// stack map 0 up to the first entry.
		if stackMap.PCs[0] > f.PC {
			if stackMap.Values[0] == 0 {
				stackMap.PCs[0] = f.PC
			} else {
				stackMap.PCs = append([]uint64{f.PC}, stackMap.PCs...)
				stackMap.Values = append([]int32{0}, stackMap.Values...)
			}
		}

		// Index -2 means "not a safe point" (i.e., no stack
		// map).
		for i, v := range stackMap.Values {
			if v == -2 {
				if stackMap.Missing == nil {
					stackMap.Missing = make([]bool, len(stackMap.Values))
				}
				stackMap.Missing[i] = true
			}
		}
	}

	return Liveness{stackMap, args, locals}, nil
}
