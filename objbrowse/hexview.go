// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"

	"github.com/aclements/objbrowse/internal/obj"
	"github.com/aclements/objbrowse/internal/symtab"
)

type HexView struct {
	fi     *FileInfo
	symTab *symtab.Table
}

func NewHexView(fi *FileInfo, symTab *symtab.Table) *HexView {
	return &HexView{fi, symTab}
}

type HexViewJS struct {
	Addr   AddrJS
	Data   string
	Relocs []HexViewRelocJS
	RTypes []string
}

type HexViewRelocJS struct {
	Offset uint64 `json:"O"` // Offset *within* data (may be negative)
	Bytes  byte   `json:"B"`
	Type   int    `json:"T"` // Index into RTypes
	Sym    string `json:"S,omitempty"`
	Addend int64  `json:"A,omitempty"`
}

func (v *HexView) DecodeSym(data obj.Data) (interface{}, error) {
	// TODO: Return just the length and fetch the raw data on
	// demand using XHR.

	// Convert relocations to JSON.
	relocs := make([]HexViewRelocJS, data.R.Len())
	syms := v.symTab.Syms()
	var rtypes []string
	rtypeMap := make(map[string]int)

	var r obj.Reloc
	for i := 0; i < data.R.Len(); i++ {
		data.R.Get(i, &r)

		typ := r.Type.String()
		typei, ok := rtypeMap[typ]
		if !ok {
			typei = len(rtypes)
			rtypes = append(rtypes, typ)
			rtypeMap[typ] = typei
		}

		var sym string
		if r.Symbol >= 0 && int(r.Symbol) < len(syms) {
			sym = syms[r.Symbol].Name
		}
		// TODO: Addend could be too large for JSON.
		relocs[i] = HexViewRelocJS{r.Offset - data.Addr, r.Size, typei, sym, r.Addend}
	}

	return HexViewJS{AddrJS(data.Addr), fmt.Sprintf("%x", data.P), relocs, rtypes}, nil
}
