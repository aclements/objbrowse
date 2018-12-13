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
	"strconv"
	"strings"

	"github.com/aclements/objbrowse/internal/asm"
	"github.com/aclements/objbrowse/internal/functab"
	"github.com/aclements/objbrowse/internal/obj"
	"github.com/aclements/objbrowse/internal/ssa"
	"github.com/aclements/objbrowse/internal/symtab"
)

var (
	httpFlag = flag.String("http", "localhost:0", "HTTP service address (e.g., ':6060')")
)

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

	state := open()
	state.serve()
}

type state struct {
	bin        obj.Obj
	symTab     *symtab.Table
	pcToFunc   map[uint64]*functab.Func
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

	// Collect function info.
	pcToFunc := make(map[uint64]*functab.Func)
	pclntab, ok := symTab.Name("runtime.pclntab")
	if ok {
		data, err := bin.SymbolData(pclntab)
		if err != nil {
			log.Fatal(err)
		}
		funcTab, err := functab.NewFuncTab(data, bin.(obj.Mem))
		if err != nil {
			log.Fatal(err)
		}
		for _, fn := range funcTab.Funcs {
			pcToFunc[fn.PC] = fn

		}
	}

	// TODO: Do something with the error.
	sourceView, _ := NewSourceView(&FileInfo{bin})

	return &state{bin, symTab, pcToFunc, sourceView}
}

func (s *state) serve() {
	ln, err := net.Listen("tcp", *httpFlag)
	if err != nil {
		log.Fatalf("failed to create server socket: %v", err)
	}
	http.HandleFunc("/", s.httpMain)
	http.Handle("/objbrowse.js", http.FileServer(http.Dir("")))
	http.Handle("/sourceview.js", http.FileServer(http.Dir("")))
	http.Handle("/asmview.js", http.FileServer(http.Dir("")))
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

var tmplMain = template.Must(template.New("").Parse(`
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

	Insts  []Disasm
	LastPC AddrJS

	Liveness interface{} `json:",omitempty"`

	SourceView interface{} `json:",omitempty"`
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
		fmt.Println(w, "unknown symbol")
		return
	}

	data, err := s.bin.SymbolData(sym)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	insts := asm.DisasmX86_64(data, sym.Value)

	if true { // TODO
		bbs, err := asm.BasicBlocks(insts)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
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
		disasm := inst.GoSyntax(s.symTab.SymName)
		op, args := parse(disasm)
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
	l, err := s.liveness(sym, insts)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	info.Liveness = l

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

func parse(disasm string) (op string, args []string) {
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

var tmplSym = template.Must(template.New("").Parse(`
<html>
<head><title>{{$.Title}}</title></head>
<body>
<style>
  html, body { margin: 0px; padding: 0px; }

  body { font-family: sans-serif; }

  .highlight { background: #c6eaff; }
  .pos {
    text-align: right;
    vertical-align: top;
    font-family: monospace;
    color: #888;
    user-select: none;
    padding-right: 0.5em;
  }
  td.pos + td:not(.pos) { border-left: #eee 1px solid; }

  .disasm { border-spacing: 0; }
  .disasm td { padding: 0 .5em; }
  .disasm th { padding: 0 .5em; }
  .disasm tr:hover { background: #def8ff; }
  .disasm .flag { text-align: center; }

  .asm-inst { white-space: nowrap; }

  .sv-path { text-align: left; padding-top: 1em; }
  .sv-error { color: #ff0000; }
  .sv-src { font-family: monospace; white-space: pre-wrap; padding-left: 0.5em; }
</style>
<svg width="0" height="0" viewBox="0 0 0 0">
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
<script src="https://code.jquery.com/jquery-3.3.1.slim.min.js"></script>
<script src="/sourceview.js"></script>
<script src="/asmview.js"></script>
<script src="/objbrowse.js"></script>
<script>render(document.body, {{$}})</script>
</body></html>
`))
