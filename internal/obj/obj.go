// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package obj

import (
	"debug/dwarf"
	"fmt"
	"io"

	"github.com/aclements/objbrowse/internal/arch"
)

type Data struct {
	// Addr is the address at which this data starts.
	Addr uint64

	// P stores the raw byte data.
	P []byte

	// R stores the relocations applied to this Data.
	//
	// This may include relocations outside or partially outside
	// of this Data's address range.
	//
	// This is never nil, so it's always safe to call methods on
	// R.
	R Relocs
}

// Mem represents a sparse memory map.
type Mem interface {
	// Data returns the data at ptr in the memory map. If size
	// exceeds the size of the data at ptr, the result will be
	// smaller than size. If ptr isn't in the memory map at all,
	// the result will be nil.
	//
	// If there are no relocations on this data, Relocs will be
	// nil.
	Data(ptr, size uint64) (Data, error)
}

// TODO: Making an Obj also a Mem is not well defined for relocatable
// files. E.g., in an unlinked object, every section can have address
// 0. Even a fully linked object may have non-load sections that don't
// have a defined address.
//
// Maybe an object is a collection of Mems? A loadable object would
// have its loaded Mem, but every symbol could independently say which
// Mem (segment or section) it's from and every reference could say
// what it's against.
//
// Right now the only use of Mem.Data is to follow the funcdata
// pointer. There could be a general "follow this pointer" that
// understands some basic relocations.

type Obj interface {
	Mem
	Info() ObjInfo
	Symbols() (Symbols, error)
	SymbolData(i SymID) (Data, error)
	DWARF() (*dwarf.Data, error)
}

type ObjInfo struct {
	// Arch is the machine architecture of this object file, or
	// nil if unknown.
	Arch *arch.Arch
}

// A SymID uniquely identifies a symbol within an object file. Symbols
// within an object file are always numbered compactly from 0.
//
// This does not necessarily correspond to the symbol indexing scheme
// used by a given object format.
//
// Some formats can have multiple symbol tables (e.g., ELF). These
// tables will be combined in a single global index space.
type SymID int

// Symbols represents the symbol table of an object file. If an object
// file has more than one symbol table, they will be combined.
type Symbols interface {
	// Len returns the number of symbols. Symbols are numbered
	// starting at 0.
	Len() SymID

	// Get fills *s with the ith symbol.
	Get(i SymID, s *Sym)
}

type Sym struct {
	Name        string
	Value, Size uint64
	Kind        SymKind
	// Local indicates this symbol's name is only meaningful
	// within its compilation unit.
	Local bool
	// HasAddr indicates this symbol's Value is a meaningful
	// address in the loaded object.
	HasAddr bool
}

type SymKind uint8

const (
	SymUnknown  SymKind = '?'
	SymText             = 'T'
	SymData             = 'D'
	SymROData           = 'R'
	SymBSS              = 'B'
	SymUndef            = 'U'
	SymAbsolute         = 'A'
)

// Relocs is a sequence of relocations.
type Relocs interface {
	// Len returns the number of relocations in this sequence.
	Len() int

	// Get fills *r with the ith relocation.
	Get(i int, r *Reloc)
}

type noRelocsType struct{}

var noRelocs Relocs = noRelocsType{}

func (noRelocsType) Len() int            { return 0 }
func (noRelocsType) Get(i int, r *Reloc) { panic("out of bounds") }

type Reloc struct {
	// Offset is the address where this Reloc is applied.
	Offset uint64
	// Size is the size of the relocation target in bytes.
	Size byte
	// Type is the relocation type. This determines how to
	// calculate the value that would be stored at Offset.
	Type RelocType
	// Symbol is the target of this Reloc, or -1 if Type does not
	// have a symbol as an input.
	Symbol SymID
	// Addend is the addend input to Type, if any.
	Addend int64
}

type RelocType interface {
	String() string
}

type unknownRelocType struct {
	val int
}

func (u unknownRelocType) String() string {
	return fmt.Sprintf("unknown(%d)", u.val)
}

// Open attempts to open r as a known object file format.
func Open(r io.ReaderAt) (Obj, error) {
	if f, err := openElf(r); err == nil {
		return f, nil
	}
	if f, err := openPE(r); err == nil {
		return f, nil
	}
	return nil, fmt.Errorf("unrecognized object file format")
}
