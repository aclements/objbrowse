// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package functab

import "sort"

type PCData struct {
	fi  *fileInfo
	pc  uint64
	raw []byte
}

func (p PCData) Decode() PCTable {
	d := decoder{nil, 0, p.raw, 0}
	pc := p.pc
	val := int32(-1)
	var tab PCTable
	for i := 0; ; i++ {
		vdelta := d.Varint()
		if vdelta == 0 && i != 0 {
			break
		}
		val += int32(vdelta)

		tab.PCs = append(tab.PCs, pc)
		tab.Values = append(tab.Values, val)

		pc += d.Uvarint() * uint64(p.fi.pcQuantum)
	}
	tab.PCs = append(tab.PCs, pc)
	return tab
}

// A PCTable represents a mapping from program counters to int32
// values. This is a range based structure, so for each i, 0 <= i <
// len(Values), all PCs in the range [PCs[i], PCs[i+1]) have value
// Values[i].
//
// If Missing is non-nil, it marks ranges that are missing from the
// table. If Missing[i] is true, then Values[i] is meaningless.
type PCTable struct {
	PCs     []uint64
	Values  []int32
	Missing []bool
}

func (tab PCTable) Lookup(pc uint64) (val int32, ok bool) {
	pcs := tab.PCs
	if len(pcs) == 0 || pc < pcs[0] || pc >= pcs[len(pcs)-1] {
		return 0, false
	}
	i := sort.Search(len(pcs), func(i int) bool {
		return pcs[i] > pc
	})
	if tab.Missing != nil && tab.Missing[i-1] {
		return 0, false
	}
	return tab.Values[i-1], true
}
