// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"os"
	"strings"

	"github.com/aclements/objbrowse/internal/asm"
	"github.com/aclements/objbrowse/internal/obj"
	"github.com/aclements/objbrowse/internal/ssa"
	"github.com/aclements/objbrowse/internal/symtab"
)

type AsmView struct {
	fi     *FileInfo
	symTab *symtab.Table

	liveness *LivenessOverlay
}

func NewAsmView(fi *FileInfo, symTab *symtab.Table) (*AsmView, error) {
	return &AsmView{fi, symTab, NewLivenessOverlay(fi, symTab)}, nil
}

type AsmViewJS struct {
	Insts  []Disasm
	LastPC AddrJS

	Liveness interface{} `json:",omitempty"`
}

type Disasm struct {
	PC      AddrJS
	Op      string
	Args    []string
	Control ControlJS
}

type ControlJS struct {
	Type        asm.ControlType
	Conditional bool
	TargetPC    AddrJS
}

func (v *AsmView) DecodeSym(sym obj.Sym, data []byte) (interface{}, error) {
	var info AsmViewJS

	insts, err := asm.Disasm(v.fi.Obj.Info().Arch, data, sym.Value)
	if err != nil {
		return nil, err
	}

	if true { // TODO
		bbs, err := asm.BasicBlocks(insts)
		if err != nil {
			return nil, err
		}

		f := ssa.SSA(insts, bbs)
		f.Fprint(os.Stdout)
	}

	//var lines []string
	var disasms []Disasm
	for i := 0; i < insts.Len(); i++ {
		inst := insts.Get(i)
		// TODO: Often the address lookups are for type.*,
		// go.string.*, or go.func.*. These are pretty
		// useless. We should at least link to the right place
		// in a hex dump. It would be way better if we could
		// do something like printing the string or resolving
		// the pointer in the funcval.
		disasm := inst.GoSyntax(v.symTab.SymName)
		op, args := parseAsm(disasm)
		control := inst.Control()
		//r, w := inst.Effects()

		//lines = append(lines, fmt.Sprintf("%s %x %x", disasm, r, w))
		disasms = append(disasms, Disasm{
			PC:   AddrJS(inst.PC()),
			Op:   op,
			Args: args,
			Control: ControlJS{
				Type:        control.Type,
				Conditional: control.Conditional,
				TargetPC:    AddrJS(control.TargetPC),
			},
		})
		info.LastPC = AddrJS(inst.PC() + uint64(inst.Len()))
	}
	info.Insts = disasms

	// Process liveness information.
	l, err := v.liveness.liveness(sym, insts)
	if err != nil {
		// TODO: Show this error, but don't block assembly on it.
		return nil, err
	}
	info.Liveness = l

	return &info, nil
}

func parseAsm(disasm string) (op string, args []string) {
	i := strings.Index(disasm, " ")
	// Include prefixes in op. In Go syntax, these are followed by
	// a semicolon.
	for i > 0 && disasm[i-1] == ';' {
		j := strings.Index(disasm[i+1:], " ")
		if j == -1 {
			i = -1
		} else {
			i += 1 + j
		}
	}
	if i == -1 {
		return disasm, []string{}
	}
	op, disasm = disasm[:i], disasm[i+1:]
	args = strings.Split(disasm, ", ")
	return
}
