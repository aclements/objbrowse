// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"regexp"
	"strconv"

	"github.com/aclements/go-obj/obj"
)

type View interface {
	Name() string
	View(entity interface{}) http.HandlerFunc
}

type server struct {
	Obj obj.File

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

	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.FS(static)))
	mux.Handle("/syms", http.HandlerFunc(s.serveSyms))
	mux.Handle("/sym/", http.HandlerFunc(s.serveSym))
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

func (s *server) serveSyms(w http.ResponseWriter, req *http.Request) {
	n := s.Obj.NumSyms()
	names := make([]string, 0, n)
	for i := obj.SymID(0); i < n; i++ {
		sym := s.Obj.Sym(i)
		names = append(names, sym.Name)
	}
	serveJSON(w, names)
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

	sym := s.Obj.Sym(obj.SymID(id))
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
