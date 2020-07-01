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
	syms []obj.Sym
	addr []obj.SymID
	name map[string]obj.SymID
}

// NewTable creates a new table for syms.
func NewTable(symbols obj.Symbols) *Table {
	syms := make([]obj.Sym, symbols.Len())
	for i := range syms {
		symbols.Get(obj.SymID(i), &syms[i])
	}

	// Put syms in address order for fast address lookup.
	addr := make([]obj.SymID, len(syms))
	for i := range addr {
		addr[i] = obj.SymID(i)
	}
	sort.Slice(addr, func(i, j int) bool {
		si, sj := &syms[addr[i]], &syms[addr[j]]

		// Put undefined symbols before defined symbols so we
		// can trim them off.
		//
		// TODO: Strip using HasAddr and remove that check in
		// Addr.
		cati, catj := 0, 0
		if si.Kind == obj.SymUndef {
			cati = -1
		}
		if sj.Kind == obj.SymUndef {
			catj = -1
		}
		if cati != catj {
			return cati < catj
		}

		// Sort by symbol address.
		vi, vj := si.Value, sj.Value
		if vi != vj {
			return vi < vj
		}

		// Secondary sort by name.
		return si.Name < sj.Name
	})

	// Trim undefined symbols.
	for len(addr) > 0 && syms[addr[0]].Kind == obj.SymUndef {
		addr = addr[1:]
	}

	// Create name map for fast name lookup.
	name := make(map[string]obj.SymID)
	for i, s := range syms {
		name[s.Name] = obj.SymID(i)
	}

	return &Table{syms, addr, name}
}

// Syms returns all symbols in Table. The returned slice can be
// indexed by SymID. The caller must not modify the returned slice.
func (t *Table) Syms() []obj.Sym {
	return t.syms
}

// Name returns the symbol with the given name.
func (t *Table) Name(name string) (obj.SymID, bool) {
	i, ok := t.name[name]
	if !ok {
		i = -1
	}
	return i, ok
}

// Addr returns the symbol containing addr.
func (t *Table) Addr(addr uint64) (obj.SymID, bool) {
	i := sort.Search(len(t.addr), func(i int) bool {
		return addr < t.syms[t.addr[i]].Value
	}) - 1
	if i < 0 {
		return -1, false
	}
	// There may be multiple symbols at this address. Pick the
	// "best" based on some heuristics.
	best, bestZeroSize := -1, -1
	for j, symi := range t.addr[i:] {
		sym := &t.syms[symi]
		if sym.Value > addr {
			break
		}
		if !sym.HasAddr {
			// This symbol's value isn't an address. For
			// example, this is an absolute symbol or a
			// TLS symbol.
			//
			// TODO: Strip these in NewTable.
			continue
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
	return -1, false
}

// SymName returns the name and base of the symbol containing addr. It
// returns "", 0 if no symbol contains addr.
//
// This ignores symbols at address 0, since those tend to not be
// "real" symbols and can cause small integers to get symbolized.
//
// This is useful for x/arch disassembly functions.
func (t *Table) SymName(addr uint64) (name string, base uint64) {
	symID, ok := t.Addr(addr)
	if !ok {
		return "", 0
	}
	sym := &t.syms[symID]
	if sym.Value == 0 {
		return "", 0
	}
	return sym.Name, sym.Value
}
