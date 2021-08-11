// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package src

import (
	"bytes"
	"io"
	"io/fs"
	"sync"
)

// LineCache reads ranges of lines from a set of files, using caching
// line position information.
type LineCache struct {
	lock sync.Mutex

	// files maps from source path to the offsets of the beginning of
	// each line.
	files map[string][]int64
}

// Get returns the data for nLines lines beginning at line lineStart for
// the given file path. Line numbers are 1-based. If lineStart is less
// than 1, this starts at line 1.
func (c *LineCache) Get(fs fs.FS, path string, lineStart, nLines int) ([]byte, error) {
	if lineStart < 1 {
		nLines += lineStart - 1
		lineStart = 1
	}
	if nLines <= 0 {
		return []byte{}, nil
	}

	c.lock.Lock()
	locked := true
	defer func() {
		if locked {
			c.lock.Unlock()
		}
	}()

	if c.files == nil {
		c.files = make(map[string][]int64)
	}

	// Get the cached line offsets.
	offsets := c.files[path]
	if offsets == nil {
		var err error
		offsets, err = makeLineOffsets(fs, path)
		if err != nil {
			return nil, err
		}
		c.files[path] = offsets
	}
	c.lock.Unlock()
	locked = false

	if lineStart-1 >= len(offsets) {
		return []byte{}, nil
	}
	if lineStart-1+nLines >= len(offsets) {
		nLines = len(offsets) - (lineStart - 1) - 1
	}
	start := offsets[lineStart-1]
	end := offsets[lineStart-1+nLines]
	buf := make([]byte, end-start)

	// Read the lines.
	f, err := fs.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	if err := readAt(f, buf, start); err != nil {
		return nil, err
	}
	return buf, nil
}

func makeLineOffsets(fs fs.FS, path string) ([]int64, error) {
	f, err := fs.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	// Start with the offset of the first line.
	offsets := []int64{0}

	var buf [4096]byte
	var pos int64
	for {
		n, err := f.Read(buf[:])

		// Find line breaks in this block.
		block := buf[:n]
		for len(block) > 0 {
			nl := bytes.IndexByte(block, '\n')
			if nl != -1 {
				offsets = append(offsets, pos+int64(nl)+1)
			} else {
				nl = len(block) - 1
			}
			pos += int64(nl) + 1
			block = block[nl+1:]
		}

		if err == io.EOF {
			break
		} else if err != nil {
			return nil, err
		}
	}

	// Add one more offset for EOF if the last line didn't end in \n.
	if len(offsets) > 0 && offsets[len(offsets)-1] != pos {
		offsets = append(offsets, pos)
	}
	return offsets, nil
}

// readAt is like io.ReaderAt.ReadAt, but works on any file. The file
// must be positioned at the start.
func readAt(f io.Reader, buf []byte, off int64) error {
	var n int
	var err error
	if f2, ok := f.(io.ReaderAt); ok {
		// Use ReadAt to read directly into the buffer.
		n, err = f2.ReadAt(buf, off)
	} else if f2, ok := f.(io.ReadSeeker); ok {
		// Seek, then read into the buffer.
		if _, err := f2.Seek(off, io.SeekStart); err != nil {
			return err
		}
		n, err = f2.Read(buf)
	} else {
		// Yuck. Discard until we get to off.
		_, err = io.CopyN(io.Discard, f, off)
		if err != nil {
			return err
		}
		n, err = f.Read(buf)
	}

	if n == len(buf) {
		return nil
	}
	if err != nil && err != io.EOF {
		return err
	}
	return io.ErrUnexpectedEOF
}
