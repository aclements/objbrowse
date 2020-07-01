// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package obj

import (
	"debug/dwarf"
	"debug/elf"
	"fmt"
	"io"
	"sort"
	"sync"

	"github.com/aclements/objbrowse/internal/arch"
)

type elfFile struct {
	elf      *elf.File
	sections map[*elf.Section]*elfSection

	syms     []elf.Symbol
	dynStart SymID // syms index of first dynamic symbol
}

type elfSection struct {
	sect *elf.Section

	// Decoded and sorted relocations applied to this section.
	relocs struct {
		srcs []*elfRelSection // REL or RELA sections that apply to this section

		load  sync.Once
		relas []elf.Rela64

		baseSymIDs []SymID // If nil, use baseSymID
		baseSymID  SymID

		err error
	}
}

func openElf(r io.ReaderAt) (Obj, error) {
	elfF, err := elf.NewFile(r)
	if err != nil {
		return nil, err
	}

	f := &elfFile{elf: elfF}

	// Load symbols from both symbol sections so we can assign
	// them global indexes. Note that the same symbol can appear
	// in both tables. Clients need to deal with that.
	staticSyms, err := f.elf.Symbols()
	if err != nil && err != elf.ErrNoSymbols {
		return nil, err
	}
	f.syms = append(f.syms, staticSyms...)
	f.dynStart = SymID(len(f.syms))
	dynSyms, err := f.elf.DynamicSymbols()
	if err != nil && err != elf.ErrNoSymbols {
		return nil, err
	}
	f.syms = append(f.syms, dynSyms...)
	elfSynthesizeSizes(f.syms, f.elf.Sections)

	// Populate section map.
	f.sections = make(map[*elf.Section]*elfSection)
	for _, sect := range f.elf.Sections[1:] {
		f.sections[sect] = &elfSection{sect: sect}
	}

	// Map relocation sections to the sections they apply to.
	//
	// There's nothing stopping ELF from containing more than one
	// of each symbol table. The debug/elf package only returns
	// the first it finds of each, so we can only handle
	// relocation sections against these.
	symtab := f.elf.SectionByType(elf.SHT_SYMTAB)
	dynsym := f.elf.SectionByType(elf.SHT_DYNSYM)
	for _, sect := range f.elf.Sections[1:] {
		switch sect.Type {
		case elf.SHT_RELA, elf.SHT_REL:
			if sect.Info < 0 || int(sect.Info) >= len(f.elf.Sections) ||
				sect.Link <= 0 || int(sect.Link) >= len(f.elf.Sections) {
				// This is an malformed relocation
				// section. Ignore.
				break
			}

			// Figure out mapping from per-symbol-table
			// indexes to global indexes.
			relsyms := f.elf.Sections[sect.Link]
			var baseSymID SymID
			if relsyms == symtab {
				baseSymID = 0
			} else if relsyms == dynsym {
				baseSymID = f.dynStart
			} else {
				// We only return symbols from these
				// two sections, so we can't represent
				// relocations that have targets from
				// some other section.
				break
			}

			ers := &elfRelSection{elf: f.elf, sect: sect, baseSymID: baseSymID}

			// Attach the relocation section to the
			// sections it applies to.
			if sect.Info != 0 {
				target := f.sections[f.elf.Sections[sect.Info]]
				target.relocs.srcs = append(target.relocs.srcs, ers)
			} else {
				// This applies to "all" sections.
				// E.g., .rela.dyn is a .so.
				for _, es := range f.sections {
					if es.sect.Flags&elf.SHF_ALLOC == 0 {
						continue
					}
					es.relocs.srcs = append(es.relocs.srcs, ers)
				}
			}
		}
	}

	return f, nil
}

func elfSynthesizeSizes(syms []elf.Symbol, sects []*elf.Section) {
	// Sort by address (without destroying order).
	addr := make([]int, 0, len(syms))
	for i := range syms {
		if elfHasAddr(&syms[i]) {
			addr = append(addr, i)
		}
	}
	sort.Slice(addr, func(i, j int) bool {
		si, sj := &syms[addr[i]], &syms[addr[j]]
		if si.Section != sj.Section {
			return si.Section < sj.Section
		}
		return si.Value < sj.Value
	})

	// Assign addresses to zero-sized symbols within each section.
	for len(addr) != 0 {
		// Collect symbols that have the same value and
		// section. Most of the time we'll get groups of 1,
		// but sometimes there are multiple names for the same
		// address (especially in shared objects).
		s1 := &syms[addr[0]]
		group := 1
		anyZero := s1.Size == 0
		for group < len(addr) {
			s2 := &syms[addr[group]]
			if s1.Value != s2.Value || s1.Section != s2.Section {
				break
			}
			if s1.Size == 0 {
				anyZero = true
			}
			group++
		}
		if !anyZero {
			// They all have sizes. Move on.
			addr = addr[group:]
			continue
		}

		// Compute the size of these symbols.
		var size uint64
		if group == len(addr) || s1.Section != syms[addr[group]].Section {
			// Cap the symbols at the end of the section.
			if 0 < s1.Section && int(s1.Section) < len(sects) {
				sect := sects[s1.Section]
				size = sect.Addr + sect.Size - s1.Value
			}
		} else {
			size = syms[addr[group]].Value - s1.Value
		}

		// Apply this size to all zero-sized symbols in this group.
		for _, symi := range addr[:group] {
			if syms[symi].Size == 0 {
				syms[symi].Size = size
			}
		}
		addr = addr[group:]
	}
}

// elfHasAddr returns true if sym's value is a meaningful address in
// the loaded object's virtual address space.
func elfHasAddr(sym *elf.Symbol) bool {
	switch sym.Section {
	case elf.SHN_UNDEF, elf.SHN_ABS:
		return false
	}
	switch elf.ST_TYPE(sym.Info) {
	case elf.STT_FILE, elf.STT_TLS:
		// STT_FILE symbols should also be absolute,
		// but we check just in case. STT_TLS symbols
		// have TLS-relative addresses, not regular
		// addresses.
		return false
	}
	return true
}

var elfToArch = map[elf.Machine]*arch.Arch{
	elf.EM_X86_64: arch.AMD64,
	elf.EM_386:    arch.I386,
	// Update elfRelocSize if you add a machine type here.
}

func (f *elfFile) Info() ObjInfo {
	return ObjInfo{
		elfToArch[f.elf.Machine],
	}
}

func (f *elfFile) Data(ptr, size uint64) (Data, error) {
	// Look up the section containing ptr.
	//
	// TODO: This whole operation is poorly defined. If this is a
	// loadable object and this is over the loaded address space,
	// we should check the program headers, not the section
	// headers. On a non-loadable object, this is meaningless (and
	// sections may even overlap).
	for _, sect := range f.elf.Sections {
		end := sect.Addr + sect.Size
		if sect.Addr <= ptr && ptr < end {
			// Found it. Limit size.
			if ptr+size > end {
				size = end - ptr
			}
			return f.sectData(sect, ptr, size)
		}
	}
	return Data{}, nil
}

func (f *elfFile) Symbols() (Symbols, error) {
	return &elfSymbols{f.elf, f.syms}, nil
}

type elfSymbols struct {
	elf  *elf.File
	syms []elf.Symbol
}

func (t *elfSymbols) Len() SymID {
	return SymID(len(t.syms))
}

func (t *elfSymbols) Get(i SymID, s *Sym) {
	esym := t.syms[i]

	kind := SymUnknown
	switch esym.Section {
	case elf.SHN_UNDEF:
		kind = SymUndef
	case elf.SHN_COMMON:
		kind = SymBSS
	case elf.SHN_ABS:
		kind = SymAbsolute
	default:
		if esym.Section < 0 || esym.Section >= elf.SectionIndex(len(t.elf.Sections)) {
			// Leave unknown.
			break
		}
		sect := t.elf.Sections[esym.Section]
		switch sect.Flags & (elf.SHF_WRITE | elf.SHF_ALLOC | elf.SHF_EXECINSTR) {
		case elf.SHF_ALLOC | elf.SHF_EXECINSTR:
			kind = SymText
		case elf.SHF_ALLOC:
			kind = SymROData
		case elf.SHF_ALLOC | elf.SHF_WRITE:
			kind = SymData
		}
	}
	local := elf.ST_BIND(esym.Info) == elf.STB_LOCAL
	hasAddr := elfHasAddr(&esym)

	*s = Sym{esym.Name, esym.Value, esym.Size, kind, local, hasAddr}
}

func (f *elfFile) SymbolData(i SymID) (Data, error) {
	s := f.syms[i]
	sect := f.elf.Sections[s.Section]
	if s.Value < sect.Addr {
		return Data{}, fmt.Errorf("symbol %q starts before section %q", s.Name, sect.Name)
	}
	return f.sectData(sect, s.Value, s.Size)
}

func (f *elfFile) DWARF() (*dwarf.Data, error) {
	return f.elf.DWARF()
}

func (f *elfFile) sectData(sect *elf.Section, ptr, size uint64) (Data, error) {
	out := Data{Addr: ptr, P: make([]byte, size), R: noRelocs}
	if sect.Type != elf.SHT_NOBITS {
		pos := ptr - sect.Addr
		flen := size
		if flen > sect.Size-pos {
			flen = sect.Size - pos
		}
		_, err := sect.ReadAt(out.P[:flen], int64(pos))
		if err != nil {
			return Data{}, err
		}
	}

	// Get relocations.
	relocs, err := f.sectRelocs(sect, ptr, size)
	if err != nil {
		return Data{}, err
	}
	if relocs == nil {
		return out, nil
	}
	out.R = relocs
	return out, err
}
