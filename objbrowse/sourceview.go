// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bufio"
	"debug/dwarf"
	"fmt"
	"io"
	"os"
	"sort"

	"github.com/aclements/objbrowse/internal/obj"
)

type SourceView struct {
	dw     *dwarf.Data
	ranges []CURange
}

type CURange struct {
	Low, High uint64
	CU        *dwarf.Entry
}

func NewSourceView(fi *FileInfo) (*SourceView, error) {
	// Load the DWARF.
	dw, err := fi.Obj.DWARF()
	if err != nil {
		return nil, err
	}

	// Create an address index for the CUs.
	var ranges []CURange
	dr := dw.Reader()
	for {
		ent, err := dr.Next()
		if ent == nil || err != nil {
			break
		}

		if ent.Tag != dwarf.TagCompileUnit {
			dr.SkipChildren()
			continue
		}

		rs, err := dw.Ranges(ent)
		if err != nil {
			continue
		}

		for _, r := range rs {
			ranges = append(ranges, CURange{r[0], r[1], ent})
		}
	}

	// Sort address index.
	sort.Slice(ranges, func(i, j int) bool {
		return ranges[i].Low < ranges[j].Low
	})

	return &SourceView{dw, ranges}, nil
}

func (v *SourceView) addrToCU(addr uint64) *dwarf.Entry {
	i := sort.Search(len(v.ranges), func(i int) bool {
		return addr < v.ranges[i].Low
	}) - 1
	if i < 0 {
		return nil
	}
	cu := v.ranges[i]
	if cu.Low <= addr && addr < cu.High {
		return cu.CU
	}
	return nil
}

type SourceViewJS struct {
	Blocks []SourceViewBlock
}

type SourceViewBlock struct {
	Path  string
	Start int
	Text  []string // Excludes trailing \n
	PCs   [][][2]AddrJS
	Error string `json:",omitempty"`
}

func (v *SourceView) DecodeSym(fi *FileInfo, sym obj.Sym) (interface{}, error) {
	// contextLines is the number of extra lines to include around
	// every source line. 0 means no context.
	const contextLines = 5
	// mergeSlack is the maximum number of extra lines to include
	// between neighboring blocks.
	const mergeSlack = 5
	// TODO(austin): Maybe the UI should have a way to get more
	// source context on demand (or this could send whole files).

	// Find sym.
	cu := v.addrToCU(sym.Value)
	if cu == nil {
		return nil, fmt.Errorf("no DWARF data for symbol %s", sym.Name)
	}

	// Get line table.
	lr, err := v.dw.LineReader(cu)
	if err != nil {
		return nil, err
	}

	// Decode the line table for this PC range.
	var line, nextLine dwarf.LineEntry
	if err = lr.SeekPC(sym.Value, &line); err == dwarf.ErrUnknownPC {
		return nil, fmt.Errorf("no line table for symbol %s", sym.Name)
	} else if err != nil {
		return nil, err
	}

	// Collect line ranges and PCs.
	end := sym.Value + sym.Size
	type rang struct {
		file     string
		from, to int // [from, to)
	}
	var ranges []rang
	type pcKey struct {
		file string
		line int
	}
	pcMap := map[pcKey][][2]uint64{}
	for line.Address < end {
		ranges = append(ranges, rang{line.File.Name, line.Line - contextLines, line.Line + contextLines + 1})

		if err = lr.Next(&nextLine); err == io.EOF {
			break
		} else if err != nil {
			return nil, err
		}

		pck := pcKey{line.File.Name, line.Line}
		pcRanges := pcMap[pck]
		if len(pcRanges) > 0 && pcRanges[len(pcRanges)-1][1] == line.Address {
			// Extend existing range.
			pcRanges[len(pcRanges)-1][1] = nextLine.Address
		} else {
			// Add a new PC range.
			pcMap[pck] = append(pcRanges, [2]uint64{line.Address, nextLine.Address})
		}

		line = nextLine
	}

	// Sort and merge lines ranges.
	sort.Slice(ranges, func(i, j int) bool {
		if ranges[i].file != ranges[j].file {
			return ranges[i].file < ranges[j].file
		}
		return ranges[i].from < ranges[j].from
	})
	ranges2 := ranges[:1]
	for _, r := range ranges[1:] {
		prev := &ranges2[len(ranges2)-1]
		if prev.file != r.file || prev.to+mergeSlack < r.from {
			// New range.
			ranges2 = append(ranges2, r)
		} else {
			// Merge ranges.
			prev.to = r.to
		}
	}
	ranges = ranges2

	// Sort and merge PC ranges.
	for pck, pcRanges := range pcMap {
		sort.Slice(pcRanges, func(i, j int) bool {
			return pcRanges[i][0] < pcRanges[j][0]
		})
		pcRanges2 := pcRanges[:1]
		for _, r := range pcRanges[1:] {
			prev := &pcRanges2[len(pcRanges2)-1]
			if prev[1] != r[0] {
				pcRanges2 = append(pcRanges2, r)
			} else {
				prev[1] = r[1]
			}
		}
		pcMap[pck] = pcRanges2
	}

	// Fetch source text.
	//
	// TODO: Check mtimes and warn if file differs.
	var blocks []SourceViewBlock
	var f *os.File
	var s *bufio.Scanner
	var fName string
	var lineNo int
	for _, r := range ranges {
		if f == nil || r.file != fName {
			f.Close()

			fName = r.file
			f, err = os.Open(fName)
			if err != nil {
				if r.file != fName {
					blocks = append(blocks, SourceViewBlock{Path: r.file, Error: err.Error()})
				}
				f, s = nil, nil
				continue
			}
			s, lineNo = bufio.NewScanner(f), 1
		}

		// Skip to the block.
		for ; lineNo < r.from && s.Scan(); lineNo++ {
		}

		// Read the block.
		var text []string
		var lineRanges [][][2]AddrJS
		start := lineNo
		for ; lineNo < r.to && s.Scan(); lineNo++ {
			text = append(text, s.Text())

			var pcRanges [][2]AddrJS
			for _, pcr := range pcMap[pcKey{fName, lineNo}] {
				pcRanges = append(pcRanges, [2]AddrJS{AddrJS(pcr[0]), AddrJS(pcr[1])})
			}
			lineRanges = append(lineRanges, pcRanges)
		}

		if err := s.Err(); err != nil {
			blocks = append(blocks, SourceViewBlock{Path: r.file, Error: err.Error()})
		} else if len(text) > 0 {
			blocks = append(blocks, SourceViewBlock{Path: r.file, Start: start, Text: text, PCs: lineRanges})
		}
	}
	f.Close()

	return SourceViewJS{Blocks: blocks}, nil
}
