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

type PCTable struct {
	PCs    []uint64
	Values []int32
}

func (tab PCTable) Lookup(pc uint64) (val int32, ok bool) {
	pcs := tab.PCs
	if len(pcs) == 0 || pc < pcs[0] || pc >= pcs[len(pcs)-1] {
		return 0, false
	}
	i := sort.Search(len(pcs), func(i int) bool {
		return pcs[i] > pc
	})
	return tab.Values[i-1], true
}
