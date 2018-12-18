// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"github.com/aclements/objbrowse/internal/asm"
	"github.com/aclements/objbrowse/internal/obj"
)

type Liveness struct {
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

func (s *state) liveness(sym obj.Sym, insts asm.Seq) (interface{}, error) {
	fn := s.pcToFunc[sym.Value]
	if fn == nil {
		return nil, nil
	}

	// TODO: Perhaps more of this knowledge should be in functab.
	var l Liveness
	arch := s.bin.Info().Arch
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
