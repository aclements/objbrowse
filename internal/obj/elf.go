// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package obj

import (
	"debug/dwarf"
	"debug/elf"
	"fmt"
	"io"

	"github.com/aclements/objbrowse/internal/arch"
)

type elfFile struct {
	elf *elf.File
}

func openElf(r io.ReaderAt) (Obj, error) {
	f, err := elf.NewFile(r)
	if err != nil {
		return nil, err
	}
	return &elfFile{f}, nil
}

var elfToArch = map[elf.Machine]*arch.Arch{
	elf.EM_X86_64: arch.AMD64,
	elf.EM_386:    arch.I386,
}

func (f *elfFile) Info() ObjInfo {
	return ObjInfo{
		elfToArch[f.elf.Machine],
	}
}

func (f *elfFile) Data(ptr, size uint64) ([]byte, error) {
	// Look up the section containing ptr.
	//
	// TODO: This should probably come from the program headers,
	// not the section headers. Then it's meaningless on an
	// unlinked object, but that has relocations, so that's
	// probably meaningless anyway.
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
	return nil, nil
}

func (f *elfFile) Symbols() ([]Sym, error) {
	syms, err := f.elf.Symbols()
	if err != nil {
		return nil, err
	}

	var out []Sym
	for _, s := range syms {
		kind := SymUnknown
		hasAddr := true
		switch s.Section {
		case elf.SHN_UNDEF:
			kind = SymUndef
			hasAddr = false
		case elf.SHN_COMMON:
			kind = SymBSS
		case elf.SHN_ABS:
			kind = SymAbsolute
			hasAddr = false
		default:
			if s.Section < 0 || s.Section >= elf.SectionIndex(len(f.elf.Sections)) {
				// Ignore symbol.
				continue
			}
			sect := f.elf.Sections[s.Section]
			switch sect.Flags & (elf.SHF_WRITE | elf.SHF_ALLOC | elf.SHF_EXECINSTR) {
			case elf.SHF_ALLOC | elf.SHF_EXECINSTR:
				kind = SymText
			case elf.SHF_ALLOC:
				kind = SymROData
			case elf.SHF_ALLOC | elf.SHF_WRITE:
				kind = SymData
			}
		}
		local := elf.ST_BIND(s.Info) == elf.STB_LOCAL
		switch elf.ST_TYPE(s.Info) {
		case elf.STT_FILE, elf.STT_TLS:
			// STT_FILE symbols should also be absolute,
			// but we check just in case. STT_TLS symbols
			// have TLS-relative addresses, not regular
			// addresses.
			hasAddr = false
		}

		sym := Sym{s.Name, s.Value, s.Size, kind, local, hasAddr, int(s.Section)}
		out = append(out, sym)
	}
	synthesizeSizes(out)
	return out, nil
}

func (f *elfFile) SymbolData(s Sym) ([]byte, error) {
	sect := f.elf.Sections[s.section]
	if s.Value < sect.Addr {
		return nil, fmt.Errorf("symbol %q starts before section %q", s.Name, sect.Name)
	}
	return f.sectData(sect, s.Value, s.Size)
}

func (f *elfFile) sectData(sect *elf.Section, ptr, size uint64) ([]byte, error) {
	out := make([]byte, size)
	pos := ptr - sect.Addr
	if pos >= sect.Size {
		return out, nil
	}
	flen := size
	if flen > sect.Size-pos {
		flen = sect.Size - pos
	}
	_, err := sect.ReadAt(out[:flen], int64(pos))
	return out, err
}

func (f *elfFile) DWARF() (*dwarf.Data, error) {
	return f.elf.DWARF()
}
