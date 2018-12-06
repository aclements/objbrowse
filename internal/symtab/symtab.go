// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package symtab

import (
	"sort"

	"github.com/aclements/objbrowse/internal/obj"
)

// Table facilitates fast symbol lookup.
type Table struct {
	addr []obj.Sym
	name map[string]int
}

// NewTable creates a new table for syms.
func NewTable(syms []obj.Sym) *Table {
	// Put syms in address order for fast address lookup.
	sort.Slice(syms, func(i, j int) bool {
		// Put undefined symbols before defined symbols so we
		// can trim them off.
		cati, catj := 0, 0
		if syms[i].Kind == obj.SymUndef {
			cati = -1
		}
		if syms[j].Kind == obj.SymUndef {
			catj = -1
		}
		if cati != catj {
			return cati < catj
		}

		// Sort by symbol address.
		vi, vj := syms[i].Value, syms[j].Value
		if vi != vj {
			return vi < vj
		}

		// Secondary sort by name.
		return syms[i].Name < syms[j].Name
	})

	// Trim undefined symbols.
	for len(syms) > 0 && syms[0].Kind == obj.SymUndef {
		syms = syms[1:]
	}

	// Create name map for fast name lookup.
	name := make(map[string]int)
	for i, s := range syms {
		name[s.Name] = i
	}

	return &Table{syms, name}
}

// Syms returns all symbols in Table in address order. The caller must
// not modify the returned slice.
func (t *Table) Syms() []obj.Sym {
	return t.addr
}

// Name returns the symbol with the given name.
func (t *Table) Name(name string) (obj.Sym, bool) {
	if i, ok := t.name[name]; ok {
		return t.addr[i], true
	}
	return obj.Sym{}, false
}

// Addr returns the symbol containing addr.
func (t *Table) Addr(addr uint64) (obj.Sym, bool) {
	i := sort.Search(len(t.addr), func(i int) bool {
		return addr < t.addr[i].Value
	}) - 1
	if i < 0 {
		return obj.Sym{}, false
	}
	// There may be multiple symbols at this address. Pick the
	// "best" based on some heuristics.
	best, bestZeroSize := -1, -1
	for j, sym := range t.addr[i:] {
		if sym.Value > addr {
			break
		}
		if best == -1 && addr < sym.Value+sym.Size {
			best = i + j
		}
		if bestZeroSize == -1 && sym.Size == 0 {
			bestZeroSize = i + j
		}
	}
	// Prefer symbols with a size, but take zero-sized symbols if
	// we must (as long as it's not the last symbol).
	switch {
	case best != -1:
		return t.addr[best], true
	case bestZeroSize != -1 && bestZeroSize != len(t.addr)-1:
		return t.addr[bestZeroSize], true
	}
	return obj.Sym{}, false
}

// SymName returns the name and base of the symbol containing addr. It
// returns "", 0 if no symbol contains addr.
//
// This is useful for x/arch disassembly functions.
func (t *Table) SymName(addr uint64) (name string, base uint64) {
	if sym, ok := t.Addr(addr); ok {
		return sym.Name, sym.Value
	}
	return "", 0
}
