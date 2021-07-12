// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"net/http"
	"strings"

	"github.com/aclements/go-obj/asm"
	"github.com/aclements/go-obj/obj"
)

// TODO: Symbolize and link references.

// TODO: Potential overlays: control flow, liveness, DWARF info for
// variables (might be better as an operand annotation that only appears
// for a selected operand), profiling info, data flow/aliasing (might be
// better as an operand annotation), extra information for resolved
// symbols (Go string or func object contents, offsets in global
// structures).
//
// Possibly PC-based overlays can apply to anything with PC ranges, if
// there's a way to combine overlays at different PCs. E.g., you could
// show control flow in the source view, too, by just combining all of
// the edges from the PC ranges of a given line.

type AsmView struct {
	f obj.File
}

func NewAsmView(s *server) *AsmView {
	return &AsmView{s.Obj}
}

func (v *AsmView) Name() string {
	return "asm"
}

type asmViewJSON struct {
	Insts  []instJSON
	LastPC AddrJS
}

type instJSON struct {
	PC           AddrJS
	Op           string
	Args         []string
	Control      *controlJSON `json:",omitempty"`
	controlStore controlJSON  `json:""` // Inlined backing store for Control
}

type controlJSON struct {
	Type        asm.ControlType
	Conditional bool
	TargetPC    AddrJS
}

func (v *AsmView) View(entity interface{}) http.HandlerFunc {
	sym, ok := entity.(*obj.Sym)
	if !ok || sym.Kind != obj.SymText || sym.Section == nil {
		return nil
	}
	return func(w http.ResponseWriter, req *http.Request) {
		var out asmViewJSON

		data, err := sym.Data(sym.Bounds())
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		insts, err := asm.Disasm(v.f.Info().Arch, data.B, sym.Value)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		var disasms []instJSON
		for i := 0; i < insts.Len(); i++ {
			inst := insts.Get(i)
			// TODO: Often the address lookups are for type.*,
			// go.string.*, or go.func.*. These are pretty
			// useless. We should at least link to the right place
			// in a hex dump. It would be way better if we could
			// do something like printing the string or resolving
			// the pointer in the funcval.
			//disasm := inst.GoSyntax(v.symTab.SymName)
			text := inst.GoSyntax(nil)
			op, args := parseAsm(text)
			disasm := instJSON{
				PC:   AddrJS(inst.PC()),
				Op:   op,
				Args: args,
			}

			control := inst.Control()
			if control.Type != asm.ControlNone {
				disasm.Control = &disasm.controlStore
				disasm.controlStore = controlJSON{
					Type:        control.Type,
					Conditional: control.Conditional,
					TargetPC:    AddrJS(control.TargetPC),
				}
			}

			disasms = append(disasms, disasm)
			out.LastPC = AddrJS(inst.PC() + uint64(inst.Len()))
		}
		out.Insts = disasms

		serveJSON(w, out)
	}
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
