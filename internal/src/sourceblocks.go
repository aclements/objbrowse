// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package src

import (
	"debug/dwarf"
	"fmt"
	"io"
	"sort"

	"github.com/aclements/go-obj/dbg"
)

type SourceBlock struct {
	Path string

	// Func is the TagSubprogram or TagInlinedSubroutine of the function
	// or inlined function that this source code is in. This may be nil
	// if unknown.
	Func *dwarf.Entry

	LineStart int
	NLines    int

	// PCs maps from a line number to a set of PC ranges of instructions
	// that appear at that line.
	PCs map[int][]Range
}

// Range represents a range of addresses [Low, High).
type Range struct {
	Low, High uint64
}

func (r Range) String() string {
	return fmt.Sprintf("[%#x, %#x)", r.Low, r.High)
}

// SourceBlocks returns a set of source code blocks for subprogram.
// Because of inlining, there may be more than one source block.
func SourceBlocks(d *dbg.Data, subprogram dbg.Subprogram) ([]SourceBlock, error) {
	lr := d.LineReader()
	if err := lr.SeekSubprogram(subprogram, 0); err != nil {
		return nil, err
	}

	// Create a map over the inlined subroutines so we can track line
	// ranges separately for each different function. If this fails, we
	// fall back to tracking them separately per file.
	type sourceKey struct {
		fn   *dwarf.Entry // TagSubprogram, TagInlinedSubroutine, or nil
		path string       // Also key by file in case function lookup fails
	}
	type sourceRange struct {
		// For each function, we track the min-to-max line range.
		lineStart, lineEnd int

		// At each line, collect the PC ranges.
		pcs map[int][]Range
	}
	sourceRanges := make(map[sourceKey]*sourceRange)
	// sourceKeys is a list of sourceRanges keys in order of first
	// discovery as we traverse the inline tree in depth-first order.
	var sourceKeys []sourceKey

	addLine := func(fn *dwarf.Entry, file *dwarf.LineFile, line int, pcs Range) {
		sk := sourceKey{fn, file.Name}
		sr := sourceRanges[sk]
		if sr == nil {
			sr = &sourceRange{pcs: make(map[int][]Range)}
			sourceRanges[sk] = sr
			sourceKeys = append(sourceKeys, sk)
		}

		// Update the source range.
		if sr.lineStart == 0 {
			sr.lineStart, sr.lineEnd = line, line+1
		} else {
			if line < sr.lineStart {
				sr.lineStart = line
			}
			if line > sr.lineEnd {
				sr.lineEnd = line
			}
		}

		// Add a PC range.
		if pcs.Low < pcs.High {
			pcRanges := sr.pcs[line]
			if len(pcRanges) > 0 && pcRanges[len(pcRanges)-1].High == pcs.Low {
				// Extend the existing range.
				pcRanges[len(pcRanges)-1].High = pcs.High
			} else {
				// Add a new PC range.
				sr.pcs[line] = append(pcRanges, pcs)
			}
		}
	}

	var addStack func(inl *dbg.InlineSite)
	addStack = func(inl *dbg.InlineSite) {
		if inl.Caller != nil {
			addStack(inl.Caller)
			addLine(inl.Caller.Entry, inl.CallFile, inl.CallLine, Range{})
		}
	}

	for {
		line, stack := lr.Line, lr.Stack
		if err := lr.Next(); err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}

		// line ranges from line.Address to lr.Line.Address.
		if line.EndSequence {
			// This isn't actually a range. Move on to the next sequence.
			continue
		}

		// Add lines. We start from the outermost inlined frame so that
		// the order of source blocks is a depth-first traversal of the
		// inline tree.
		if stack != nil {
			addStack(stack)
			addLine(stack.Entry, line.File, line.Line, Range{line.Address, lr.Line.Address})
		} else {
			// We weren't able to resolve PC to a function. Just key by
			// file name and do the best we can.
			addLine(nil, line.File, line.Line, Range{line.Address, lr.Line.Address})
		}
	}

	// Finalize each sourceRange into a SourceBlock.
	blocks := make([]SourceBlock, 0, len(sourceKeys))
	for _, sk := range sourceKeys {
		sr := sourceRanges[sk]
		block := SourceBlock{
			Path:      sk.path,
			Func:      sk.fn,
			LineStart: sr.lineStart,
			NLines:    sr.lineEnd - sr.lineStart + 1,
		}
		for line, ranges := range sr.pcs {
			sr.pcs[line] = mergeRanges(ranges)
		}
		block.PCs = sr.pcs

		blocks = append(blocks, block)
	}

	return blocks, nil
}

func mergeRanges(ranges []Range) []Range {
	sort.Slice(ranges, func(i, j int) bool {
		return ranges[i].Low < ranges[j].Low
	})

	// Merge overlapping ranges.
	o, i := 0, 1
	for ; i < len(ranges); i++ {
		if ranges[o].High >= ranges[i].Low {
			// Merge the ranges. o might subsume i.
			if ranges[i].High > ranges[o].High {
				ranges[o].High = ranges[i].High
			}
		} else {
			o++
			ranges[o] = ranges[i]
		}
	}
	return ranges[:o+1]
}
