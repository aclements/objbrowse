// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package functab

import (
	"bytes"
	"debug/dwarf"
	"encoding/binary"
	"fmt"
	"strings"

	"github.com/aclements/objbrowse/internal/obj"
)

type FuncTab struct {
	Funcs []*Func
	EndPC uint64

	// PCDATA and FUNCDATA indexes
	Indexes map[string]int64

	_PCDATA_StackMapIndex       int
	_FUNCDATA_ArgsPointerMaps   int
	_FUNCDATA_LocalsPointerMaps int
}

type Func struct {
	PC       uint64
	Name     string
	PCSP     PCData
	PCData   []PCData
	FuncData []FuncData
	ft       *FuncTab
}

type symtabHdr struct {
	Magic     uint32
	_         uint16
	PCQuantum uint8
	PtrSize   uint8
}

type fileInfo struct {
	mmap      obj.Obj
	order     binary.ByteOrder
	ptrSize   int
	pcQuantum uint8
}

// NewFuncTab decodes a Go function table from data, which should be
// the contents of the "runtime.pclntab" symbol in the object file
// given by obj.
func NewFuncTab(data []byte, obj obj.Obj) (*FuncTab, error) {
	var err error
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
	fi := &fileInfo{obj, d.order, d.ptrSize, hdr.PCQuantum}

	ft := new(FuncTab)

	// Read func PC/offset table.
	//
	// See cmd/link/internal/ld/pcln.go:pclntab
	nfunc := d.Ptr()
	ft.Funcs = make([]*Func, nfunc)
	offsets := make([]uint64, nfunc)
	for i := range offsets {
		d.Ptr() // PC (will read from func later)
		offsets[i] = d.Ptr()
	}
	ft.EndPC = d.Ptr()
	d.Uint32() // fileTabOffset

	// Extract the PCDATA and FUNCDATA index definitions.
	dw, err := obj.DWARF()
	if err != nil {
		return nil, err
	}
	ft.Indexes, err = getDataIndexes(dw)
	if err != nil {
		return nil, err
	}
	fetchIndex := func(name string, out *int) {
		val, ok := ft.Indexes[name]
		if !ok && err == nil {
			err = fmt.Errorf("missing definition of %s", name)
		}
		*out = int(val)
	}
	fetchIndex("_PCDATA_StackMapIndex", &ft._PCDATA_StackMapIndex)
	fetchIndex("_FUNCDATA_ArgsPointerMaps", &ft._FUNCDATA_ArgsPointerMaps)
	fetchIndex("_FUNCDATA_LocalsPointerMaps", &ft._FUNCDATA_LocalsPointerMaps)
	if err != nil {
		return nil, err
	}

	// Read func structures.
	for i := range ft.Funcs {
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

		fn := &Func{pc, name, pcsp, pcdata, funcdata, ft}
		ft.Funcs[i] = fn
	}

	return ft, nil
}

func getDataIndexes(dw *dwarf.Data) (map[string]int64, error) {
	// Look for global runtime._(FUNCDATA|PCDATA)_* constants.
	r := dw.Reader()
	indexes := make(map[string]int64)
	for {
		ent, err := r.Next()
		if err != nil {
			return nil, err
		} else if ent == nil {
			break
		}
		switch ent.Tag {
		default:
			r.SkipChildren()

		case dwarf.TagCompileUnit:
			// Process children

		case dwarf.TagConstant:
			name, ok := ent.Val(dwarf.AttrName).(string)
			if !ok {
				break
			}
			if !(strings.HasPrefix(name, "runtime._FUNCDATA_") ||
				strings.HasPrefix(name, "runtime._PCDATA_")) {
				break
			}
			name = name[len("runtime."):]
			val, ok := ent.Val(dwarf.AttrConstValue).(int64)
			if !ok {
				break
			}
			indexes[name] = val
		}
	}

	// Check that we got the ones we need.
	for _, want := range []string{"_PCDATA_StackMapIndex",
		"_FUNCDATA_ArgsPointerMaps", "_FUNCDATA_LocalsPointerMaps"} {
		if _, ok := indexes[want]; !ok {
			return nil, fmt.Errorf("missing definition of %s", want)
		}
	}

	return indexes, nil
}

type Liveness struct {
	Index        PCTable
	Args, Locals []Bitmap
}

func (f Func) Liveness() (Liveness, error) {
	if len(f.PCData) <= f.ft._PCDATA_StackMapIndex {
		return Liveness{}, nil
	}

	// Fetch the pointer bitmaps.
	args, err := f.FuncData[f.ft._FUNCDATA_ArgsPointerMaps].StackMap()
	if err != nil {
		return Liveness{}, nil
	}
	locals, err := f.FuncData[f.ft._FUNCDATA_LocalsPointerMaps].StackMap()
	if err != nil {
		return Liveness{}, nil
	}

	// Fetch the stack map index.
	stackMap := f.PCData[f.ft._PCDATA_StackMapIndex].Decode()

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
