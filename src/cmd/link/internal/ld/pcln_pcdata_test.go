// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ld

import (
	"bytes"
	"testing"
)

func TestCompactPCPosition(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte
		fileSize int64
		want     []byte
		base     uint32
	}{
		{
			name:  "line zero on a real PC is observable",
			input: []byte{62, 2, 59, 0, 60, 3, 59, 2, 62, 1, 0},
			want:  []byte{62, 5, 59, 2, 62, 1, 0},
		},
		{
			name:     "rebase file indices and keep unknown file",
			input:    []byte{0, 1, 132, 1, 2, 6, 2, 137, 1, 1, 132, 1, 3, 0},
			fileSize: 1024,
			want:     []byte{0, 1, 2, 2, 6, 2, 7, 1, 2, 3, 0},
			base:     65,
		},
		{
			name:     "rebasing would make a bad index look valid",
			input:    []byte{132, 1, 2, 0},
			fileSize: 65,
			want:     []byte{132, 1, 2, 0},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, base := compactPCPosition(test.input, test.fileSize)
			if !bytes.Equal(got, test.want) || base != test.base {
				t.Fatalf("compactPCPosition = (%v, %d), want (%v, %d)", got, base, test.want, test.base)
			}
		})
	}
}

func TestDefaultPCData(t *testing.T) {
	for _, test := range []struct {
		data     []byte
		textSize int64
		want     bool
	}{
		{[]byte{0, 4, 0}, 16, true},
		{[]byte{0, 4, 0}, 17, false},
		{[]byte{1, 4, 0}, 16, false},
		{[]byte{0, 3, 2, 1, 0}, 16, false},
	} {
		if got := defaultPCData(test.data, 4, test.textSize); got != test.want {
			t.Errorf("defaultPCData(%v, 4, %d) = %t, want %t", test.data, test.textSize, got, test.want)
		}
	}
}
