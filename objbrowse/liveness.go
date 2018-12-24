// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"log"

	"github.com/aclements/objbrowse/internal/asm"
	"github.com/aclements/objbrowse/internal/functab"
	"github.com/aclements/objbrowse/internal/obj"
	"github.com/aclements/objbrowse/internal/symtab"
)

type LivenessOverlay struct {
	fi       *FileInfo
	pcToFunc map[uint64]*functab.Func
}

func NewLivenessOverlay(fi *FileInfo, symTab *symtab.Table) *LivenessOverlay {
	// Collect function info.
	pcToFunc := make(map[uint64]*functab.Func)
	pclntab, ok := symTab.Name("runtime.pclntab")
	if ok {
		data, err := fi.Obj.SymbolData(pclntab)
		if err != nil {
			log.Fatal(err)
		}
		funcTab, err := functab.NewFuncTab(data, fi.Obj)
		if err != nil {
			log.Fatal(err)
		}
		for _, fn := range funcTab.Funcs {
			pcToFunc[fn.PC] = fn
		}
	}

	return &LivenessOverlay{fi, pcToFunc}
}

type LivenessJS struct {
	PtrSize int

	// The difference between SPOff and Varp and Argp for this
	// architecture.
	VarpDelta, ArgpDelta int

	// SP offsets.
	SPOff []LivenessRangeJS

	// Bitmap indexes.
	Indexes []LivenessRangeJS

	// Hex-encoded locals and args bitmaps
	Locals, Args []string
}

type LivenessRangeJS struct {
	Start AddrJS `json:"start"`
	End   AddrJS `json:"end"`
	Val   int32  `json:"val"`
}

func pcTableToJS(t functab.PCTable) []LivenessRangeJS {
	var out []LivenessRangeJS
	for i, val := range t.Values {
		if t.Missing != nil && t.Missing[i] {
			continue
		}
		out = append(out, LivenessRangeJS{AddrJS(t.PCs[i]), AddrJS(t.PCs[i+1]), val})
	}
	return out
}

func (o *LivenessOverlay) liveness(sym obj.Sym, insts asm.Seq) (interface{}, error) {
	fn := o.pcToFunc[sym.Value]
	if fn == nil {
		return nil, nil
	}

	// TODO: Perhaps more of this knowledge should be in functab.
	var l LivenessJS
	arch := o.fi.Obj.Info().Arch
	l.PtrSize = arch.PtrSize
	l.VarpDelta = -l.PtrSize
	l.ArgpDelta = arch.MinFrameSize

	// Decode bitmaps.
	liveness, err := fn.Liveness()
	if err != nil {
		return nil, err
	}
	if len(liveness.Locals) == 0 {
		// No liveness data.
		return nil, nil
	}
	for _, bitmap := range liveness.Locals {
		l.Locals = append(l.Locals, bitmap.Hex())
	}
	for _, bitmap := range liveness.Args {
		l.Args = append(l.Args, bitmap.Hex())
	}
	l.Indexes = pcTableToJS(liveness.Index)

	// Decode SP offsets.
	l.SPOff = pcTableToJS(fn.PCSP.Decode())

	return l, nil
}
