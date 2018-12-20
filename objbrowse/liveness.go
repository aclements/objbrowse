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

	// SP offset for each instruction.
	SPOff []int

	// Bitmap index for each instruction.
	Indexes []int

	// Hex-encoded locals and args bitmaps
	Locals, Args []string
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

	// Decode SP offset and index at each instruction.
	pcsp := fn.PCSP.Decode()
	for i := 0; i < insts.Len(); i++ {
		inst := insts.Get(i)
		spOffset, ok := pcsp.Lookup(inst.PC())
		if !ok {
			spOffset = -1
		}
		l.SPOff = append(l.SPOff, int(spOffset))

		stackIdx, ok := liveness.Index.Lookup(inst.PC())
		if stackIdx < -1 || len(liveness.Index.PCs) == 0 {
			// Not a safe-point, or no liveness info.
			stackIdx = -1
		} else if stackIdx == -1 {
			// By convention, this actually means index 0.
			stackIdx = 0
		}
		l.Indexes = append(l.Indexes, int(stackIdx))
	}

	return l, nil
}
