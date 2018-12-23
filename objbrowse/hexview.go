// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"

	"github.com/aclements/objbrowse/internal/obj"
)

type HexView struct {
	fi *FileInfo
}

func NewHexView(fi *FileInfo) *HexView {
	return &HexView{fi}
}

type HexViewJS struct {
	Addr AddrJS
	Data string
}

func (v *HexView) DecodeSym(sym obj.Sym, data []byte) (interface{}, error) {
	// TODO: Return just the length and fetch the raw data on
	// demand using XHR.
	return HexViewJS{AddrJS(sym.Value), fmt.Sprintf("%x", data)}, nil
}
