// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// TODO: Implement relocs.

package obj

import (
	"debug/dwarf"
	"debug/pe"
	"fmt"
	"io"
	"sort"

	"github.com/aclements/objbrowse/internal/arch"
)

type peFile struct {
	pe        *pe.File
	imageBase uint64
	sizes     []uint64
}

func openPE(r io.ReaderAt) (Obj, error) {
	f, err := pe.NewFile(r)
	if err != nil {
		return nil, err
	}

	var imageBase uint64
	switch oh := f.OptionalHeader.(type) {
	case *pe.OptionalHeader32:
		imageBase = uint64(oh.ImageBase)
	case *pe.OptionalHeader64:
		imageBase = oh.ImageBase
	default:
		return nil, fmt.Errorf("PE header has unexpected type")
	}

	// Assign symbol sizes.
	sizes := peSynthesizeSizes(f.Symbols, f.Sections)
	return &peFile{f, imageBase, sizes}, nil
}

func peSynthesizeSizes(syms []*pe.Symbol, sects []*pe.Section) []uint64 {
	// Sort by address (without destroying order).
	addr := make([]int, len(syms))
	for i := range addr {
		addr[i] = i
	}
	sort.Slice(addr, func(i, j int) bool {
		si, sj := syms[addr[i]], syms[addr[j]]
		if si.SectionNumber != sj.SectionNumber {
			return si.SectionNumber < sj.SectionNumber
		}
		return si.Value < sj.Value
	})

	// Assign addresses to symbols.
	sizes := make([]uint64, len(syms))
	for i, symi := range addr {
		sym := syms[symi]
		if sym.SectionNumber <= 0 {
			// Not an addressable section, so no
			// meaningful sizes.
			continue
		}
		if i == len(addr) || sym.SectionNumber != syms[addr[i+1]].SectionNumber {
			// Cap the symbol at the end of the section.
			if int(sym.SectionNumber)-1 < len(sects) {
				sect := sects[int(sym.SectionNumber)-1]
				if sym.Value < sect.VirtualSize {
					sizes[symi] = uint64(sect.VirtualSize - sym.Value)
				}
			}
		} else {
			sizes[symi] = uint64(syms[addr[i+1]].Value - sym.Value)
		}
	}

	return sizes
}

var peToArch = map[uint16]*arch.Arch{
	pe.IMAGE_FILE_MACHINE_AMD64: arch.AMD64,
	pe.IMAGE_FILE_MACHINE_I386:  arch.I386,
}

func (f *peFile) Info() ObjInfo {
	return ObjInfo{
		peToArch[f.pe.Machine],
	}
}

func (f *peFile) Data(ptr, size uint64) (Data, error) {
	panic("not implemented")
}

func (f *peFile) Symbols() (Symbols, error) {
	return (*peSymbols)(f), nil
}

type peSymbols peFile

func (f *peSymbols) Len() SymID {
	return SymID(len(f.pe.Symbols))
}

func (f *peSymbols) Get(i SymID, sym *Sym) {
	const (
		IMAGE_SYM_UNDEFINED = 0
		IMAGE_SYM_ABSOLUTE  = -1
		IMAGE_SYM_DEBUG     = -2

		IMAGE_SYM_CLASS_STATIC = 3

		IMAGE_SCN_CNT_CODE               = 0x20
		IMAGE_SCN_CNT_INITIALIZED_DATA   = 0x40
		IMAGE_SCN_CNT_UNINITIALIZED_DATA = 0x80
		IMAGE_SCN_MEM_WRITE              = 0x80000000
	)

	s := f.pe.Symbols[i]

	*sym = Sym{s.Name, uint64(s.Value), 0, SymUnknown, false, false}
	switch s.SectionNumber {
	case IMAGE_SYM_UNDEFINED:
		sym.Kind = SymUndef
	case IMAGE_SYM_ABSOLUTE:
		sym.Kind = SymAbsolute
	case IMAGE_SYM_DEBUG:
		// Leave unknown
	default:
		if int(s.SectionNumber)-1 < 0 || int(s.SectionNumber)-1 >= len(f.pe.Sections) {
			// Leave unknown
			break
		}
		sect := f.pe.Sections[int(s.SectionNumber)-1]
		c := sect.Characteristics
		switch {
		case c&IMAGE_SCN_CNT_CODE != 0:
			sym.Kind = SymText
		case c&IMAGE_SCN_CNT_INITIALIZED_DATA != 0:
			if c&IMAGE_SCN_MEM_WRITE != 0 {
				sym.Kind = SymData
			} else {
				sym.Kind = SymROData
			}
		case c&IMAGE_SCN_CNT_UNINITIALIZED_DATA != 0:
			sym.Kind = SymBSS
		}
		sym.Local = s.StorageClass == IMAGE_SYM_CLASS_STATIC
		sym.Value += f.imageBase + uint64(sect.VirtualAddress)
		sym.HasAddr = true
	}
}

func (f *peFile) SymbolData(i SymID) (Data, error) {
	s := f.pe.Symbols[i]
	if s.SectionNumber <= 0 || int(s.SectionNumber)-1 >= len(f.pe.Sections) {
		return Data{R: noRelocs}, nil
	}
	sect := f.pe.Sections[s.SectionNumber-1]
	if s.Value < sect.VirtualAddress {
		return Data{}, fmt.Errorf("symbol %q starts before section %q", s.Name, sect.Name)
	}
	value := f.imageBase + uint64(s.Value) + uint64(sect.VirtualAddress)
	out := Data{Addr: value, P: make([]byte, f.sizes[i]), R: noRelocs}
	if s.Value < sect.Size {
		flen := f.sizes[i]
		if flen > uint64(sect.Size-s.Value) {
			flen = uint64(sect.Size - s.Value)
		}
		_, err := sect.ReadAt(out.P[:flen], int64(s.Value))
		if err != nil {
			return Data{}, err
		}
	}
	return out, nil
}

func (f *peFile) DWARF() (*dwarf.Data, error) {
	return f.pe.DWARF()
}
