// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package src

import (
	"io"
	"strings"
	"testing"
	"testing/fstest"
)

var lines = []string{"line1", "line2", "line3"}

var lineCacheFS = fstest.MapFS{
	"file": &fstest.MapFile{
		Data: []byte(strings.Join(lines, "\n") + "\n"),
		Mode: 0o666,
	},
	"noNL": &fstest.MapFile{
		Data: []byte(strings.Join(lines, "\n")),
		Mode: 0o666,
	},
	"big": &fstest.MapFile{
		Data: []byte(strings.Repeat("x", 10000) + "\nline2\n"),
		Mode: 0o666,
	},
}

func TestLineCache(t *testing.T) {
	var lc LineCache
	check := func(path string, start, end int, want string) {
		t.Helper()
		got, err := lc.Get(lineCacheFS, path, start, end)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
			return
		}
		if want != string(got) {
			t.Errorf("%s:%d–%d: want %q, got %q", path, start, start+end, want, string(got))
		}
	}

	// Basic tests.
	check("file", 1, 3, "line1\nline2\nline3\n")
	check("file", 1, 2, "line1\nline2\n")
	check("file", 3, 1, "line3\n")

	// Weird line boundaries.
	check("file", 1, 0, "")
	check("file", -1, 1, "")
	check("file", 4, 1, "")
	check("file", 5, 1, "")

	// New newline terminator.
	check("noNL", 1, 3, "line1\nline2\nline3")
	check("noNL", 3, 1, "line3")
	check("noNL", 4, 1, "")
	check("noNL", 5, 1, "")

	// Big lines (to stress block reader).
	check("big", 2, 1, "line2\n")
}

func TestReadAt(t *testing.T) {
	f, err := lineCacheFS.Open("big")
	if err != nil {
		t.Fatal(err)
	}
	check := func(label string, f2 io.Reader) {
		t.Run(label, func(t *testing.T) {
			// Make sure the file is positioned at the beginning.
			f.(io.ReadSeeker).Seek(0, io.SeekStart)

			buf := make([]byte, 6)
			err := readAt(f2, buf, 10001)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			want := "line2\n"
			if want != string(buf) {
				t.Fatalf("want %q, got %q", want, string(buf))
			}

			// Read past EOF.
			f.(io.ReadSeeker).Seek(0, io.SeekStart)
			err = readAt(f2, buf, 10002)
			if err != io.ErrUnexpectedEOF {
				t.Fatalf("want ErrUnexpectedEOF, got %v", err)
			}
		})
	}
	_ = f.(io.ReaderAt) // Make sure it really is a ReaderAt.
	check("ReaderAt", f)
	check("ReadSeeker", readSeeker{f.(io.ReadSeeker)})
	check("Reader", reader{f})
}

type readSeeker struct {
	io.ReadSeeker
}

type reader struct {
	io.Reader
}
