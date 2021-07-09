// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"net/http"

	"github.com/aclements/go-obj/obj"
)

type HexView struct {
	f obj.File
}

func NewHexView(s *server) *HexView {
	return &HexView{s.Obj}
}

func (v *HexView) Name() string {
	return "hex"
}

type hasData interface {
	Data(addr, size uint64) (*obj.Data, error)
	Bounds() (addr, size uint64)
}

type hexViewJSON struct {
	Addr AddrJS
	Data string
}

func (v *HexView) View(entity interface{}) http.HandlerFunc {
	// TODO: Display relocations.

	entityData, ok := entity.(hasData)
	if !ok {
		return nil
	}
	return func(w http.ResponseWriter, req *http.Request) {
		data, err := entityData.Data(entityData.Bounds())
		if err != nil {
			if err, ok := err.(*obj.ErrNoData); ok {
				serveJSON(w, err.Error())
			} else {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
			return
		}
		serveJSON(w, hexViewJSON{AddrJS(data.Addr), fmt.Sprintf("%x", data.P)})
	}
}
