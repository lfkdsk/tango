// Copyright 2025 ByteDance Ltd. and/or its affiliates. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Regression for the GORoData materialize/restore pair
// (-enable-go-global-init-opt). Read-only string symbols are external
// declarations carrying go.rodata bytes; materialization renames them to
// ".old" and the restore pass must find that declaration after the full
// optimization pipeline. Options also keeps an implicit zero tail (a nil map
// field) that must not be truncated by materialization.
package main

type Options struct {
	Namespace, Subsystem, Name, Help string
	ConstLabels                      map[string]string
}

//go:noinline
func name(ns, sub, metric string) string { return ns + "_" + sub + "_" + metric }

//go:noinline
func valid(metric, help string, variables []string, constants map[string]string) int {
	var mismatch int
	if constants != nil {
		mismatch |= 1
	}
	if variables != nil {
		mismatch |= 2
	}
	if len(metric) != 5 {
		mismatch |= 4
	}
	if len(help) != 4 {
		mismatch |= 8
	}
	return mismatch
}

//go:noinline
func gauge(opts Options) int {
	return valid(name(opts.Namespace, opts.Subsystem, opts.Name), opts.Help, nil, opts.ConstLabels)
}

//go:noinline
func pick(s string, i int) byte { return s[i] }

func main() {
	opts := Options{Namespace: "n", Subsystem: "s", Name: "x", Help: "help"}
	if mismatch := gauge(opts); mismatch != 0 {
		panic(mismatch)
	}
	// The trailing map field is implicitly zero and must stay nil after
	// materialization.
	if opts.ConstLabels != nil {
		panic("implicit zero tail lost")
	}
	// Constant read-only strings of several lengths go through the bytes-only
	// materialization path; their contents and addresses must stay intact.
	strs := [...]string{"", "A", "ABCDEFGH", "ABCDEFGHABCDEFGH"}
	want := [...]int{0, 1, 8, 16}
	for i, s := range strs {
		if len(s) != want[i] {
			panic("length")
		}
		if len(s) != 0 && pick(s, len(s)-1) != 'H' && s != "A" {
			panic("content")
		}
	}
	if pick("help", 3) != 'p' {
		panic("byte load from rodata string")
	}
}
