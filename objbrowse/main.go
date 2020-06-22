// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"flag"
	"fmt"
	"html/template"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"github.com/aclements/objbrowse/internal/obj"
	"github.com/aclements/objbrowse/internal/symtab"
)

var (
	httpFlag   = flag.String("http", "localhost:0", "HTTP service address (e.g., ':6060')")
	flagStatic = flag.String("static", defaultStatic(), "`path` to static files")
)

func defaultStatic() string {
	path, err := os.Executable()
	if err != nil {
		return ""
	}
	path2, err := filepath.EvalSymlinks(path)
	if err != nil {
		path2 = path
	}
	return filepath.Dir(path2)
}

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [flags] objfile\n", os.Args[0])
		flag.PrintDefaults()
	}
	flag.Parse()
	if flag.NArg() != 1 {
		flag.Usage()
		os.Exit(2)
	}
	if *flagStatic == "" {
		fmt.Fprintf(os.Stderr, "Unable to find static resources.\nPlease provide -static flag.\n")
		os.Exit(2)
	}

	state := open()
	state.serve()
}

type state struct {
	bin        obj.Obj
	symTab     *symtab.Table
	hexView    *HexView
	asmView    *AsmView
	sourceView *SourceView
}

type FileInfo struct {
	Obj obj.Obj
}

func open() *state {
	f, err := os.Open(flag.Arg(0))
	if err != nil {
		log.Fatal(err)
	}

	bin, err := obj.Open(f)
	if err != nil {
		log.Fatal(err)
	}

	syms, err := bin.Symbols()
	if err != nil {
		log.Fatal(err)
	}

	symTab := symtab.NewTable(syms)

	// TODO: Do something with the error.
	fi := &FileInfo{bin}
	hexView := NewHexView(fi)
	asmView, _ := NewAsmView(fi, symTab)
	sourceView, _ := NewSourceView(fi)

	return &state{bin, symTab, hexView, asmView, sourceView}
}

func (s *state) serve() {
	ln, err := net.Listen("tcp", *httpFlag)
	if err != nil {
		log.Fatalf("failed to create server socket: %v", err)
	}
	http.HandleFunc("/", s.httpMain)
	fs := http.FileServer(http.Dir(*flagStatic))
	http.Handle("/objbrowse.css", fs)
	http.Handle("/objbrowse.js", fs)
	http.Handle("/hexview.js", fs)
	http.Handle("/asmview.js", fs)
	http.Handle("/sourceview.js", fs)
	http.Handle("/liveness.js", fs)
	http.HandleFunc("/s/", s.httpSym)
	addr := "http://" + ln.Addr().String()
	fmt.Printf("Listening on %s\n", addr)
	err = http.Serve(ln, nil)
	log.Fatalf("failed to start HTTP server: %v", err)
}

func (s *state) httpMain(w http.ResponseWriter, r *http.Request) {
	// TODO: Put this in a nice table.
	// TODO: Option to sort by name or address.
	// TODO: More nm-like information (type and maybe value)
	// TODO: Make hierarchical on "."
	// TODO: Filter by symbol type.
	// TODO: Filter by substring.
	// TODO: Option to demangle (do hierarchy splitting before demangling)
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	syms := s.symTab.Syms()

	if err := tmplMain.Execute(w, syms); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

var tmplMain = template.Must(template.New("").Parse(`<!DOCTYPE html>
<html><body>
{{range $s := $}}<a href="/s/{{$s.Name}}">{{printf "%#x" $s.Value}} {{printf "%c" $s.Kind}} {{$s.Name}}</a><br />{{end}}
</body></html>
`))

// AddrJS is an address for storing in JSON. It is represented in hex
// with no leading "0x".
type AddrJS uint64

func (a AddrJS) MarshalJSON() ([]byte, error) {
	buf := make([]byte, 0, 18)
	buf = append(buf, '"')
	buf = strconv.AppendUint(buf, uint64(a), 16)
	return append(buf, '"'), nil
}

type SymInfo struct {
	Title string
	Base  AddrJS

	HexView    interface{} `json:",omitempty"`
	AsmView    interface{} `json:",omitempty"`
	SourceView interface{} `json:",omitempty"`
}

func (s *state) httpSym(w http.ResponseWriter, r *http.Request) {
	// TODO: Highlight sources of data read by instruction and
	// sinks of data written by instruction.

	// TODO: Option to show dot basic block graph with cross-links
	// to assembly listing? Maybe also dominator tree? Maybe this
	// is another parallel view?

	// TODO: Have parallel views of symbols: hex dump,
	// disassembly, and source. For data symbols, just have hex
	// dump. Cross-link the views, so clicking on a line of source
	// highlights all of the assembly for the line and the hex
	// corresponding to those instructions, etc. Could have
	// further parallel views, too, like decoding hex values using
	// DWARF type information.

	// TODO: Allow selecting a range of lines and highlighting all
	// of them.

	// TODO: Support for overlaying things like profile
	// information? (Could also use this for liveness, etc.) Would
	// be nice if this were "pluggable".

	// TODO: Have a way to navigate control flow, leaving behind
	// "breadcrumbs" of sequential control flow. E.g., clicking on
	// a jump adds instructions between current position and jump
	// to a breadcrumb list and follows the jump. Clicking on a
	// ret does the same and then uses the call stack to go back
	// to where you came from. Also have a way to back up in this
	// control flow (and maybe a way to fork, probably just using
	// browser tabs).

	// TODO: Option to re-order assembly so control-flow is more
	// local. Maybe edge spring model? Or topo order?

	var info SymInfo

	symName := r.URL.Path[3:]
	info.Title = symName

	sym, ok := s.symTab.Name(symName)
	if !ok {
		fmt.Fprintln(w, "unknown symbol")
		return
	}
	info.Base = AddrJS(sym.Value)

	data, err := s.bin.SymbolData(sym)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Process HexView.
	hv, err := s.hexView.DecodeSym(sym, data)
	if err != nil {
		// TODO: Display this to the user.
		log.Print(err)
	} else {
		info.HexView = hv
	}

	// Process AsmView.
	av, err := s.asmView.DecodeSym(sym, data)
	if err != nil {
		// TODO: Display this to the user.
		log.Print(err)
	} else {
		info.AsmView = av
	}

	// Process SourceView.
	sv, err := s.sourceView.DecodeSym(&FileInfo{s.bin}, sym)
	if err != nil {
		// TODO: Display this to the user.
		log.Print(err)
	} else {
		info.SourceView = sv
	}

	if err := tmplSym.Execute(w, info); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

var tmplSym = template.Must(template.New("").Parse(`<!DOCTYPE html>
<html>
<head>
<title>{{$.Title}}</title>
<link rel="stylesheet" type="text/css" href="/objbrowse.css" />
</head>
<body>
<svg width="0" height="0" style="position:absolute">
  <defs>
    <marker id="tri" viewBox="0 0 10 10" refX="0" refY="5"
            markerUnits="userSpaceOnUse" markerWidth="10"
            markerHeight="8" orient="auto">
        <path d="M 0 0 L 10 5 L 0 10 z" fill="context-stroke"></path>
    </marker>
    <marker id="markX" viewBox="0 0 10 10" refX="5" refY="5"
            markerUnits="userSpaceOnUse" markerWidth="10"
            markerHeight="10">
        <path d="M 0 0 L 10 10 M 10 0 L 0 10" stroke="black" stroke-width="2px"></path>
    </marker>
  </defs>
</svg>
<script src="https://code.jquery.com/jquery-3.3.1.min.js"></script>
<script src="/hexview.js"></script>
<script src="/asmview.js"></script>
<script src="/sourceview.js"></script>
<script src="/liveness.js"></script>
<script src="/objbrowse.js"></script>
<script>render(document.body, {{$}})</script>
</body></html>
`))
