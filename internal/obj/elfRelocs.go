// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package obj

import (
	"debug/elf"
	"encoding/binary"
	"sort"
	"sync"
)

type elfRelocType struct {
	typ  RelocType
	size byte
}

const elfRelocMaxSize = 16

var elfRelocTypes = map[elf.Machine]map[uint32]elfRelocType{
	elf.EM_X86_64: map[uint32]elfRelocType{
		uint32(elf.R_X86_64_NONE):            {elf.R_X86_64_NONE, 0},
		uint32(elf.R_X86_64_64):              {elf.R_X86_64_64, 8},
		uint32(elf.R_X86_64_PC32):            {elf.R_X86_64_PC32, 4},
		uint32(elf.R_X86_64_GOT32):           {elf.R_X86_64_GOT32, 4},
		uint32(elf.R_X86_64_PLT32):           {elf.R_X86_64_PLT32, 4},
		uint32(elf.R_X86_64_COPY):            {elf.R_X86_64_COPY, 0},
		uint32(elf.R_X86_64_GLOB_DAT):        {elf.R_X86_64_GLOB_DAT, 8},
		uint32(elf.R_X86_64_JMP_SLOT):        {elf.R_X86_64_JMP_SLOT, 8},
		uint32(elf.R_X86_64_RELATIVE):        {elf.R_X86_64_RELATIVE, 8},
		uint32(elf.R_X86_64_GOTPCREL):        {elf.R_X86_64_GOTPCREL, 4},
		uint32(elf.R_X86_64_32):              {elf.R_X86_64_32, 4},
		uint32(elf.R_X86_64_32S):             {elf.R_X86_64_32S, 4},
		uint32(elf.R_X86_64_16):              {elf.R_X86_64_16, 2},
		uint32(elf.R_X86_64_PC16):            {elf.R_X86_64_PC16, 2},
		uint32(elf.R_X86_64_8):               {elf.R_X86_64_8, 1},
		uint32(elf.R_X86_64_PC8):             {elf.R_X86_64_PC8, 1},
		uint32(elf.R_X86_64_DTPMOD64):        {elf.R_X86_64_DTPMOD64, 8},
		uint32(elf.R_X86_64_DTPOFF64):        {elf.R_X86_64_DTPOFF64, 8},
		uint32(elf.R_X86_64_TPOFF64):         {elf.R_X86_64_TPOFF64, 8},
		uint32(elf.R_X86_64_TLSGD):           {elf.R_X86_64_TLSGD, 4},
		uint32(elf.R_X86_64_TLSLD):           {elf.R_X86_64_TLSLD, 4},
		uint32(elf.R_X86_64_DTPOFF32):        {elf.R_X86_64_DTPOFF32, 4},
		uint32(elf.R_X86_64_GOTTPOFF):        {elf.R_X86_64_GOTTPOFF, 4},
		uint32(elf.R_X86_64_TPOFF32):         {elf.R_X86_64_TPOFF32, 4},
		uint32(elf.R_X86_64_PC64):            {elf.R_X86_64_PC64, 8},
		uint32(elf.R_X86_64_GOTOFF64):        {elf.R_X86_64_GOTOFF64, 8},
		uint32(elf.R_X86_64_GOTPC32):         {elf.R_X86_64_GOTPC32, 4},
		uint32(elf.R_X86_64_GOT64):           {elf.R_X86_64_GOT64, 8},
		uint32(elf.R_X86_64_GOTPCREL64):      {elf.R_X86_64_GOTPCREL64, 8},
		uint32(elf.R_X86_64_GOTPC64):         {elf.R_X86_64_GOTPC64, 8},
		uint32(elf.R_X86_64_GOTPLT64):        {elf.R_X86_64_GOTPLT64, 8},
		uint32(elf.R_X86_64_PLTOFF64):        {elf.R_X86_64_PLTOFF64, 8},
		uint32(elf.R_X86_64_SIZE32):          {elf.R_X86_64_SIZE32, 4},
		uint32(elf.R_X86_64_SIZE64):          {elf.R_X86_64_SIZE64, 8},
		uint32(elf.R_X86_64_GOTPC32_TLSDESC): {elf.R_X86_64_GOTPC32_TLSDESC, 4},
		uint32(elf.R_X86_64_TLSDESC_CALL):    {elf.R_X86_64_TLSDESC_CALL, 0},
		uint32(elf.R_X86_64_TLSDESC):         {elf.R_X86_64_TLSDESC, 16},
		uint32(elf.R_X86_64_IRELATIVE):       {elf.R_X86_64_IRELATIVE, 8},
		// See https://github.com/hjl-tools/x86-psABI/wiki/X86-psABI
		uint32(elf.R_X86_64_RELATIVE64):    {elf.R_X86_64_RELATIVE64, 8}, // For x32
		uint32(elf.R_X86_64_PC32_BND):      {elf.R_X86_64_PC32_BND, 4},   // For x32; deprecated
		uint32(elf.R_X86_64_PLT32_BND):     {elf.R_X86_64_PLT32_BND, 4},  // For x32; deprecated
		uint32(elf.R_X86_64_GOTPCRELX):     {elf.R_X86_64_GOTPCRELX, 4},
		uint32(elf.R_X86_64_REX_GOTPCRELX): {elf.R_X86_64_REX_GOTPCRELX, 4},
	},

	elf.EM_386: map[uint32]elfRelocType{
		uint32(elf.R_386_NONE):          {elf.R_386_NONE, 0},
		uint32(elf.R_386_32):            {elf.R_386_32, 4},
		uint32(elf.R_386_PC32):          {elf.R_386_PC32, 4},
		uint32(elf.R_386_GOT32):         {elf.R_386_GOT32, 4},
		uint32(elf.R_386_PLT32):         {elf.R_386_PLT32, 4},
		uint32(elf.R_386_COPY):          {elf.R_386_COPY, 0},
		uint32(elf.R_386_GLOB_DAT):      {elf.R_386_GLOB_DAT, 4},
		uint32(elf.R_386_JMP_SLOT):      {elf.R_386_JMP_SLOT, 4},
		uint32(elf.R_386_RELATIVE):      {elf.R_386_RELATIVE, 4},
		uint32(elf.R_386_GOTOFF):        {elf.R_386_GOTOFF, 4},
		uint32(elf.R_386_GOTPC):         {elf.R_386_GOTPC, 4},
		uint32(elf.R_386_TLS_TPOFF):     {elf.R_386_TLS_TPOFF, 4},
		uint32(elf.R_386_TLS_IE):        {elf.R_386_TLS_IE, 4},
		uint32(elf.R_386_TLS_GOTIE):     {elf.R_386_TLS_GOTIE, 4},
		uint32(elf.R_386_TLS_LE):        {elf.R_386_TLS_LE, 4},
		uint32(elf.R_386_TLS_GD):        {elf.R_386_TLS_GD, 4},
		uint32(elf.R_386_TLS_LDM):       {elf.R_386_TLS_LDM, 4},
		uint32(elf.R_386_16):            {elf.R_386_16, 2},
		uint32(elf.R_386_PC16):          {elf.R_386_PC16, 2},
		uint32(elf.R_386_8):             {elf.R_386_8, 1},
		uint32(elf.R_386_PC8):           {elf.R_386_PC8, 1},
		uint32(elf.R_386_TLS_GD_32):     {elf.R_386_TLS_GD_32, 4},
		uint32(elf.R_386_TLS_GD_PUSH):   {elf.R_386_TLS_GD_PUSH, 4},
		uint32(elf.R_386_TLS_GD_CALL):   {elf.R_386_TLS_GD_CALL, 4},
		uint32(elf.R_386_TLS_GD_POP):    {elf.R_386_TLS_GD_POP, 4},
		uint32(elf.R_386_TLS_LDM_32):    {elf.R_386_TLS_LDM_32, 4},
		uint32(elf.R_386_TLS_LDM_PUSH):  {elf.R_386_TLS_LDM_PUSH, 4},
		uint32(elf.R_386_TLS_LDM_CALL):  {elf.R_386_TLS_LDM_CALL, 4},
		uint32(elf.R_386_TLS_LDM_POP):   {elf.R_386_TLS_LDM_POP, 4},
		uint32(elf.R_386_TLS_LDO_32):    {elf.R_386_TLS_LDO_32, 4},
		uint32(elf.R_386_TLS_IE_32):     {elf.R_386_TLS_IE_32, 4},
		uint32(elf.R_386_TLS_LE_32):     {elf.R_386_TLS_LE_32, 4},
		uint32(elf.R_386_TLS_DTPMOD32):  {elf.R_386_TLS_DTPMOD32, 4},
		uint32(elf.R_386_TLS_DTPOFF32):  {elf.R_386_TLS_DTPOFF32, 4},
		uint32(elf.R_386_TLS_TPOFF32):   {elf.R_386_TLS_TPOFF32, 4},
		uint32(elf.R_386_SIZE32):        {elf.R_386_SIZE32, 4},
		uint32(elf.R_386_TLS_GOTDESC):   {elf.R_386_TLS_GOTDESC, 4},
		uint32(elf.R_386_TLS_DESC_CALL): {elf.R_386_TLS_DESC_CALL, 0},
		uint32(elf.R_386_TLS_DESC):      {elf.R_386_TLS_DESC, 4},
		uint32(elf.R_386_IRELATIVE):     {elf.R_386_IRELATIVE, 4},
		uint32(elf.R_386_GOT32X):        {elf.R_386_GOT32X, 4},
	},
}

// elfRelSection is a decoded SHT_REL[A] section.
type elfRelSection struct {
	elf  *elf.File
	sect *elf.Section // SHT_REL[A] section

	// baseSymID is the global symbol index of the first symbol in
	// this relocation section's symbol table.
	baseSymID SymID

	load  sync.Once
	relas []elf.Rela64 // Sorted by offset
	err   error
}

func (r *elfRelSection) get(addr, size uint64) ([]elf.Rela64, error) {
	// Decode this relocations section.
	r.load.Do(func() {
		o := r.elf.ByteOrder

		data, err := r.sect.Data()
		if err != nil {
			r.err = err
			return
		}

		// Parse and canonicalize the relocations in rela64s.
		var relas []elf.Rela64
		switch {
		case r.sect.Type == elf.SHT_REL && r.elf.Class == elf.ELFCLASS32:
			relas = elfReadRel32(data, o)
		case r.sect.Type == elf.SHT_REL && r.elf.Class == elf.ELFCLASS64:
			relas = elfReadRel64(data, o)
		case r.sect.Type == elf.SHT_RELA && r.elf.Class == elf.ELFCLASS32:
			relas = elfReadRela32(data, o)
		case r.sect.Type == elf.SHT_RELA && r.elf.Class == elf.ELFCLASS64:
			relas = elfReadRela64(data, o)
		default:
			// We shouldn't have created an elfRelSection
			// for this at all.
			panic("unexpected relocation section type")
		}

		// Sort relocations by address for fast lookup and
		// range slicing.
		sort.Slice(relas, func(i, j int) bool { return relas[i].Off < relas[j].Off })

		r.relas = relas
	})

	if r.err != nil {
		return nil, r.err
	}

	// Find the relocations for this region. This is only used for
	// whole sections, so we don't have to worry about relocations
	// that partially overlap the region.
	relas := r.relas
	start := sort.Search(len(r.relas), func(i int) bool {
		return relas[i].Off >= addr
	})
	end := sort.Search(len(r.relas), func(i int) bool {
		return relas[i].Off >= addr+size
	})

	return relas[start:end], nil
}

func elfReadRel32(data []byte, o binary.ByteOrder) []elf.Rela64 {
	var out []elf.Rela64
	for len(data) >= 8 {
		off := o.Uint32(data)
		info := o.Uint32(data[4:])
		info64 := elf.R_INFO(elf.R_SYM32(info), elf.R_TYPE32(info))
		data = data[8:]
		out = append(out, elf.Rela64{uint64(off), info64, 0})
	}
	return out
}

func elfReadRel64(data []byte, o binary.ByteOrder) []elf.Rela64 {
	var out []elf.Rela64
	for len(data) >= 16 {
		off := o.Uint64(data)
		info := o.Uint64(data[8:])
		data = data[16:]
		out = append(out, elf.Rela64{off, info, 0})
	}
	return out
}

func elfReadRela32(data []byte, o binary.ByteOrder) []elf.Rela64 {
	var out []elf.Rela64
	for len(data) >= 12 {
		off := o.Uint32(data)
		info := o.Uint32(data[4:])
		info64 := elf.R_INFO(elf.R_SYM32(info), elf.R_TYPE32(info))
		add := int32(o.Uint32(data[8:]))
		data = data[12:]
		out = append(out, elf.Rela64{uint64(off), info64, int64(add)})
	}
	return out
}

func elfReadRela64(data []byte, o binary.ByteOrder) []elf.Rela64 {
	var out []elf.Rela64
	for len(data) >= 24 {
		off := o.Uint64(data)
		info := o.Uint64(data[8:])
		add := int64(o.Uint64(data[16:]))
		data = data[24:]
		out = append(out, elf.Rela64{off, info, add})
	}
	return out
}

func (f *elfFile) sectRelocs(sect *elf.Section, ptr, size uint64) (*elfRelocs, error) {
	s := f.sections[sect]
	if s == nil || len(s.relocs.srcs) == 0 {
		return nil, nil
	}
	s.relocs.load.Do(func() {
		// Load the relocations.
		var all [][]elf.Rela64
		var baseSymID []SymID
		for _, src := range s.relocs.srcs {
			relas, err := src.get(sect.Addr, sect.Size)
			if err != nil {
				s.relocs.err = err
				return
			}
			if len(relas) > 0 {
				all = append(all, relas)
				baseSymID = append(baseSymID, src.baseSymID)
			}
		}

		// In the common case, there's only one applicable
		// relas slice, and we can use it directly.
		switch len(all) {
		case 0:
			return
		case 1:
			s.relocs.relas = all[0]
			s.relocs.baseSymID = baseSymID[0]
			return
		}
		// Merge the relocations.
		var relas []elf.Rela64
		var baseSymIDs []SymID
		for i, a := range all {
			relas = append(relas, a...)
			for range a {
				baseSymIDs = append(baseSymIDs, baseSymID[i])
			}
		}
		sort.Sort(&elfRelaSorter{relas, baseSymIDs})
		s.relocs.relas = relas
		s.relocs.baseSymIDs = baseSymIDs
	})
	if s.relocs.err != nil {
		return nil, s.relocs.err
	}
	relas := s.relocs.relas

	// Position the iterator at the first relocation that overlaps
	// this range. Since relocations have different sizes, we
	// binary search for the first that *could* overlap it, the
	// linearly trim until we get a real overlap.
	types := elfRelocTypes[f.elf.Machine]
	start := sort.Search(len(relas), func(i int) bool {
		return relas[i].Off+elfRelocMaxSize >= ptr
	})
	for ; start < len(relas); start++ {
		sz := types[elf.R_TYPE64(relas[start].Info)].size
		if relas[start].Off+uint64(sz) >= ptr {
			break
		}
	}
	// Find the iterator end.
	end := sort.Search(len(relas), func(i int) bool {
		return relas[i].Off >= ptr+size
	})
	relas = relas[start:end]

	// Slice the base SymIDs likewise.
	var baseSymIDs []SymID
	if s.relocs.baseSymIDs != nil {
		baseSymIDs = s.relocs.baseSymIDs[start:end]
	}

	return &elfRelocs{types, relas, s.relocs.baseSymID, baseSymIDs}, nil
}

type elfRelaSorter struct {
	relas      []elf.Rela64
	baseSymIDs []SymID
}

func (s *elfRelaSorter) Len() int           { return len(s.relas) }
func (s *elfRelaSorter) Less(i, j int) bool { return s.relas[i].Off < s.relas[j].Off }
func (s *elfRelaSorter) Swap(i, j int) {
	s.relas[i], s.relas[j] = s.relas[j], s.relas[i]
	s.baseSymIDs[i], s.baseSymIDs[j] = s.baseSymIDs[j], s.baseSymIDs[i]
}

type elfRelocs struct {
	types      map[uint32]elfRelocType
	relas      []elf.Rela64
	baseSymID  SymID
	baseSymIDs []SymID
}

func (rs *elfRelocs) Len() int {
	return len(rs.relas)
}

func (rs *elfRelocs) Get(i int, r *Reloc) {
	rela := rs.relas[i]

	sym, typ := elf.R_SYM64(rela.Info), elf.R_TYPE64(rela.Info)
	ert, ok := rs.types[typ]
	if !ok {
		ert.typ = unknownRelocType{int(typ)}
	}

	symID := SymID(-1)
	if sym != 0 {
		// baseSymID is symbol 1 in the section.
		if rs.baseSymIDs != nil {
			symID = rs.baseSymIDs[i] + SymID(sym) - 1
		} else {
			symID = rs.baseSymID + SymID(sym) - 1
		}
	}

	*r = Reloc{rela.Off, ert.size, ert.typ, symID, rela.Addend}
}
