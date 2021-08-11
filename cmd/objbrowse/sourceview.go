// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"debug/dwarf"
	"net/http"
	"os"
	"strings"

	"github.com/aclements/go-obj/dbg"
	"github.com/aclements/go-obj/obj"
	"github.com/aclements/objbrowse/internal/src"
)

type SourceView struct {
	dw       *dwarf.Data
	dbg      *dbg.Data
	dbgError error

	lineCache src.LineCache
}

func NewSourceView(s *server) *SourceView {
	return &SourceView{dw: s.Dwarf, dbg: s.Dbg, dbgError: s.DbgError}
}

func (v *SourceView) Name() string {
	return "source"
}

type sourceViewJSON struct {
	Blocks []sourceViewBlockJSON
}

type sourceViewBlockJSON struct {
	Path  string
	Func  string
	Start int
	Text  []string // Excludes trailing \n on each line
	PCs   [][][2]AddrJS
	Error string `json:",omitempty"`
}

func (v *SourceView) View(entity interface{}) http.HandlerFunc {
	sym, ok := entity.(*obj.Sym)
	if !ok || sym.Kind != obj.SymText || sym.Section == nil {
		return nil
	}
	return func(w http.ResponseWriter, req *http.Request) {
		if v.dbgError != nil {
			http.Error(w, v.dbgError.Error(), http.StatusInternalServerError)
			return
		}

		subprogram, ok := v.dbg.AddrToSubprogram(sym.Value, dbg.CU{})
		if !ok {
			http.Error(w, "symbol address not found in debug info", http.StatusInternalServerError)
			return
		}
		blocks, err := src.SourceBlocks(v.dbg, subprogram)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		var out sourceViewJSON
		fs := os.DirFS("/")
		for _, block := range blocks {
			// Expand the source block by a few lines.
			const contextLines = 5
			start, end := block.LineStart, block.LineStart+block.NLines
			start -= contextLines
			end += contextLines
			if start < 1 {
				start = 1
			}

			bjs := sourceViewBlockJSON{
				Path:  block.Path,
				Func:  entryName(v.dw, block.Func),
				Start: start,
			}

			// Get source text.
			//
			// TODO: Check the mtimes of files and warn if they differ.
			if !strings.HasPrefix(block.Path, "/") {
				// It's not clear what we should do with relative paths.
				bjs.Error = "relative path: " + block.Path
			} else if text, err := v.lineCache.Get(fs, block.Path[1:], start, end-start); err != nil {
				bjs.Error = err.Error()
			} else {
				// Trim trailing \n
				if len(text) > 0 && text[len(text)-1] == '\n' {
					text = text[:len(text)-1]
				}
				bjs.Text = strings.Split(string(text), "\n")
			}

			// Convert PC ranges.
			bjs.PCs = make([][][2]AddrJS, end-start)
			for line, ranges := range block.PCs {
				i := line - start
				bjs.PCs[i] = make([][2]AddrJS, len(ranges))
				for j, r := range ranges {
					bjs.PCs[i][j] = [2]AddrJS{AddrJS(r.Low), AddrJS(r.High)}
				}
			}
			// Fill in missing lines with an empty slice rather than
			// sending "null" in the JSON.
			empty := [][2]AddrJS{}
			for i, ranges := range bjs.PCs {
				if ranges == nil {
					bjs.PCs[i] = empty
				}
			}

			out.Blocks = append(out.Blocks, bjs)
		}
		serveJSON(w, out)
	}
}

func entryName(dw *dwarf.Data, ent *dwarf.Entry) string {
	if name, ok := ent.Val(dwarf.AttrName).(string); ok {
		return name
	}
	// Check for an abstract origin for inlined subroutines.
	if ao, ok := ent.Val(dwarf.AttrAbstractOrigin).(dwarf.Offset); ok {
		r := dw.Reader()
		r.Seek(ao)
		if ent, err := r.Next(); err == nil {
			if name, ok := ent.Val(dwarf.AttrName).(string); ok {
				return name
			}
		}
	}
	return ""
}
