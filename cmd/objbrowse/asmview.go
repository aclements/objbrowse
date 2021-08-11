// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/aclements/go-obj/asm"
	"github.com/aclements/go-obj/obj"
	"github.com/aclements/go-obj/symtab"
)

// TODO: Symbolize and link references.

// TODO: Support selecting different styles (Plan 9, Intel, GNU).

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
	f      obj.File
	symTab *symtab.Table
}

func NewAsmView(s *server) *AsmView {
	return &AsmView{s.Obj, s.SymTab}
}

func (v *AsmView) Name() string {
	return "asm"
}

type asmViewJSON struct {
	Insts  []instJSON
	Refs   []symRefJSON
	LastPC AddrJS
}

type instJSON struct {
	PC           AddrJS
	Op           string
	Args         string       // Symbol references embedded as «%d+%x», index, offset
	Control      *controlJSON `json:",omitempty"`
	controlStore controlJSON  `json:""` // Inlined backing store for Control
}

type controlJSON struct {
	Type        string
	Conditional bool
	TargetPC    *AddrJS `json:",omitempty"`
}

type symRefJSON struct {
	ID   obj.SymID
	Name string
	Addr AddrJS
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

		// Construct a symbol lookup function. Since names aren't
		// unique, we use an identifiable format to embed the symbol ID
		// and offset so we can extract and link to them later.
		symRefs := []symRefJSON{}
		symRefMap := make(map[obj.SymID]int)
		symName := func(addr uint64) (name string, base uint64) {
			symID := v.symTab.Addr(sym.Section, addr)
			if symID == obj.NoSym {
				return "", 0
			}
			sym := v.symTab.Syms()[symID]
			ref, ok := symRefMap[symID]
			if !ok {
				ref = len(symRefs)
				symRefs = append(symRefs, symRefJSON{symID, sym.Name, AddrJS(sym.Value)})
				symRefMap[symID] = ref
			}
			offset := addr - sym.Value
			return fmt.Sprintf("«%d+%x»", ref, offset), addr
		}

		insts, err := asm.Disasm(v.f.Info().Arch, data.B, sym.Value)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		disasms := []instJSON{}
		for i := 0; i < insts.Len(); i++ {
			inst := insts.Get(i)
			// TODO: Often the address lookups are for type.*,
			// go.string.*, or go.func.*. These are pretty useless. We
			// should at least link to the right place in a hex dump. It
			// would be way better if we could do something like
			// printing the string or resolving the pointer in the
			// funcval.
			//
			// If there's a relocation here, we might need to follow it
			// to the target. I'm not sure how to do that. I might just
			// need to put any relocations on this instruction on the
			// side. Maybe relocations are just a general overlay.

			text := inst.GoSyntax(symName)
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
					Type:        control.Type.String()[len("Control"):],
					Conditional: control.Conditional,
				}
				if control.TargetPC != ^uint64(0) {
					pc := AddrJS(control.TargetPC)
					disasm.Control.TargetPC = &pc
				}
			}

			disasms = append(disasms, disasm)
			out.LastPC = AddrJS(inst.PC() + uint64(inst.Len()))
		}
		out.Refs = symRefs
		out.Insts = disasms

		serveJSON(w, out)
	}
}

func parseAsm(disasm string) (op string, args string) {
	i := strings.Index(disasm, " ")
	// Include REP prefixes in op. In Go syntax, these are followed by a
	// semicolon.
	//
	// TODO: Other prefixes, like LOCK, are just separated with spaces.
	for i > 0 && disasm[i-1] == ';' {
		j := strings.Index(disasm[i+1:], " ")
		if j == -1 {
			i = -1
		} else {
			i += 1 + j
		}
	}
	if i == -1 {
		return disasm, ""
	}
	op, args = disasm[:i], disasm[i+1:]
	return
}
