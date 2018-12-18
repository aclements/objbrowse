// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package arch

type Arch struct {
	// GoArch is the GOARCH value for this architecture.
	GoArch string

	// PtrSize is the number of bytes in a pointer.
	PtrSize int

	// MinFrameSize is the number of bytes at the bottom of every
	// stack frame except for empty leaf frames. This includes,
	// for example, space for a saved LR (because that space is
	// always reserved), but does not include the return PC pushed
	// on x86 by CALL (because that is added only on a call).
	MinFrameSize int
}

var (
	AMD64 = &Arch{"amd64", 8, 0}
	I386  = &Arch{"386", 4, 0}
)

func (a *Arch) String() string {
	if a == nil {
		return "<nil>"
	}
	return a.GoArch
}
