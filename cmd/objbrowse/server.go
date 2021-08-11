// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"debug/dwarf"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"net"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/aclements/go-obj/dbg"
	"github.com/aclements/go-obj/obj"
	"github.com/aclements/go-obj/symtab"
)

type View interface {
	Name() string
	View(entity interface{}) http.HandlerFunc
}

type server struct {
	Obj    obj.File
	SymTab *symtab.Table

	Dwarf    *dwarf.Data
	Dbg      *dbg.Data
	DbgError error // If Dbg == nil, the error loading debug info

	listener net.Listener
	mux      http.Handler

	viewMap map[string]View
	views   []View
}

func newServer(f obj.File, host string, static fs.FS) (*server, error) {
	ln, err := net.Listen("tcp", host)
	if err != nil {
		return nil, err
	}
	s := &server{Obj: f, listener: ln, viewMap: make(map[string]View)}

	// Get all symbols, synthesize missing sizes, and create a symbol table.
	syms := make([]obj.Sym, f.NumSyms())
	for i := range syms {
		syms[i] = f.Sym(obj.SymID(i))
	}
	obj.SynthesizeSizes(syms)
	s.SymTab = symtab.NewTable(syms)

	// Get debug info.
	if f, ok := f.(obj.AsDebugDwarf); ok {
		s.Dwarf, err = f.AsDebugDwarf()
		if err == nil {
			s.Dbg, err = dbg.New(s.Dwarf)
		}
		if err != nil {
			log.Printf("error reading DWARF debug info: %v", err)
			s.DbgError = err
		}
	}

	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.FS(static)))
	mux.Handle("/index", http.HandlerFunc(s.serveIndex))
	mux.Handle("/sym/", http.HandlerFunc(s.serveSym))
	// TODO: Also provide an index over sections and maybe even a view
	// over the whole object.
	s.mux = mux

	return s, nil
}

func (s *server) addView(v View) {
	if s.viewMap[v.Name()] != nil {
		panic(fmt.Errorf("conflicting View name: %s already added", v.Name()))
	}
	s.views = append(s.views, v)
	s.viewMap[v.Name()] = v
}

func (s *server) serve() error {
	return http.Serve(s.listener, s.mux)
}

type indexJSON struct {
	Views []string
	// We store the symbols as struct-of-arrays because it makes the
	// JSON representation much smaller. The client side will transpose
	// this back into objects.
	Syms struct {
		Names  []string
		Values []AddrJS
		Sizes  []uint64
		Kinds  string // Indexed by sym ID
		Views  []int  // Bit mask over Views list
	}
}

func (s *server) serveIndex(w http.ResponseWriter, req *http.Request) {
	var js indexJSON

	for _, view := range s.views {
		js.Views = append(js.Views, view.Name())
	}

	syms := s.SymTab.Syms()
	n := len(syms)
	js.Syms.Names = make([]string, n)
	js.Syms.Values = make([]AddrJS, n)
	js.Syms.Sizes = make([]uint64, n)
	var kinds strings.Builder
	js.Syms.Views = make([]int, n)
	for i, sym := range syms {
		// TODO: Option to demangle C++ names (and maybe Go names)
		js.Syms.Names[i] = sym.Name
		js.Syms.Values[i] = AddrJS(sym.Value)
		js.Syms.Sizes[i] = sym.Size
		kinds.WriteByte(byte(sym.Kind))

		viewSet := 0
		for viewI, view := range s.views {
			if view.View(&sym) != nil {
				viewSet |= 1 << viewI
			}
		}
		js.Syms.Views[i] = viewSet
	}
	js.Syms.Kinds = kinds.String()

	serveJSON(w, &js)
}

// symURLRe matches symbol queries, which must be of the form /sym/{id}/{view}.
var symURLRe = regexp.MustCompile(`^/sym/([0-9]+)/([^/]+)$`)

func (s *server) serveSym(w http.ResponseWriter, req *http.Request) {
	m := symURLRe.FindStringSubmatch(req.URL.Path)
	if m == nil {
		http.NotFound(w, req)
		return
	}

	id, err := strconv.Atoi(m[1])
	if err != nil {
		http.Error(w, "malformed symbol ID: "+err.Error(), http.StatusNotFound)
		return
	}
	if id < 0 || id >= int(s.Obj.NumSyms()) {
		http.Error(w, "unknown symbol ID", http.StatusNotFound)
		return
	}

	view, ok := s.viewMap[m[2]]
	if !ok {
		http.Error(w, "unknown view", http.StatusNotFound)
		return
	}

	// Get the symbol from the symbol table so we get any synthesized sizes.
	sym := s.SymTab.Syms()[id]
	viewer := view.View(&sym)
	if viewer == nil {
		http.Error(w, "view does not support this entity", http.StatusNotFound)
		return
	}

	viewer(w, req)
}

func serveJSON(w http.ResponseWriter, data interface{}) {
	b, err := json.Marshal(data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Add("Content-type", "application/json")
	w.Write(b)
}

// AddrJS is an address for storing in JSON. It is represented in hex
// with no leading "0x".
type AddrJS uint64

func (a AddrJS) MarshalJSON() ([]byte, error) {
	buf := make([]byte, 0, 18)
	buf = append(buf, '"')
	buf = strconv.AppendUint(buf, uint64(a), 16)
	return append(buf, '"'), nil
}

func (a AddrJS) MarshalJSONTo(buf *bytes.Buffer) error {
	buf.WriteByte('"')
	ubuf := make([]byte, 0, 18)
	ubuf = strconv.AppendUint(ubuf, uint64(a), 16)
	buf.Write(ubuf)
	buf.WriteByte('"')
	return nil
}
