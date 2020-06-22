// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"encoding/json"

	"github.com/aclements/objbrowse/internal/obj"
	"github.com/aclements/objbrowse/internal/symtab"
)

type SymView struct {
	fi     *FileInfo
	symTab *symtab.Table
}

func NewSymView(fi *FileInfo, symTab *symtab.Table) *SymView {
	return &SymView{fi, symTab}
}

type SymViewJS struct {
	Syms SymViewSymsJS
}

type SymViewSymsJS struct {
	Syms []obj.Sym
}

func (s *SymViewSymsJS) MarshalJSON() ([]byte, error) {
	// Because symbol tables can be very large, we encode SymJS
	// more compactly than the default encoding.
	//
	// TODO: This still allocates a lot more than necessary,
	// mostly just to write JSON strings. Maybe we should just do
	// our own string escaping, too.

	buf := new(bytes.Buffer)
	enc := json.NewEncoder(buf)
	buf.WriteByte('[')
	for i, sym := range s.Syms {
		if i > 0 {
			buf.WriteByte(',')
		}
		buf.WriteByte('[')
		enc.Encode(sym.Name)
		buf.WriteString(",\"")
		buf.WriteByte(byte(sym.Kind))
		buf.WriteString("\",")
		AddrJS(sym.Value).MarshalJSONTo(buf)
		buf.WriteByte(']')
	}
	buf.WriteByte(']')

	return buf.Bytes(), nil
}

func (v *SymView) Decode() (interface{}, error) {
	return &SymViewJS{SymViewSymsJS{v.symTab.Syms()}}, nil
}
