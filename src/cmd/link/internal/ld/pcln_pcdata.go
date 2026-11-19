// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ld

import (
	"bytes"
	"cmd/internal/obj"
	"cmd/link/internal/loader"
	"encoding/binary"
	"hash/maphash"
	"internal/abi"
	"math"
)

type pclnPCData struct {
	funcs []pclnFuncPCData
	extra []uint32
}

type pclnFuncPCData struct {
	pcsp, pcfile, pcline uint32
	fileBase             uint32
	start, end           int // bounds in pclnPCData.extra
}

type pctabEntry struct {
	data   []byte
	offset uint32
	next   int // 1-based index of the next entry with the same hash
}

type pctabBuilder struct {
	size    int64
	entries []pctabEntry
	seed    maphash.Seed
	heads   map[uint64]int
}

func (p *pctabBuilder) add(data []byte) uint32 {
	if len(data) == 0 {
		return 0
	}
	hash := maphash.Bytes(p.seed, data)
	head := p.heads[hash]
	for i := head; i != 0; i = p.entries[i-1].next {
		entry := &p.entries[i-1]
		if bytes.Equal(entry.data, data) {
			return entry.offset
		}
	}
	off := uint32(p.size)
	p.size += int64(len(data))
	p.entries = append(p.entries, pctabEntry{data: data, offset: off, next: head})
	p.heads[hash] = len(p.entries)
	return off
}

func collectPCData(ctxt *Link, funcs []loader.Sym, fileBytes int64) (pclnPCData, *pctabBuilder) {
	ldr := ctxt.loader
	pcs := pclnPCData{funcs: make([]pclnFuncPCData, len(funcs))}
	// Offset zero denotes an absent table.
	table := &pctabBuilder{size: 1, seed: maphash.MakeSeed(), heads: make(map[uint64]int)}
	rawOffsets := make(map[loader.Sym]uint32)
	rawOffset := func(s loader.Sym) uint32 {
		if s == 0 {
			return 0
		}
		if off, ok := rawOffsets[s]; ok {
			return off
		}
		off := table.add(ldr.Data(s))
		rawOffsets[s] = off
		return off
	}

	type positionKey struct {
		sym  loader.Sym
		file bool
	}
	type positionTable struct {
		offset, base uint32
	}
	positions := make(map[positionKey]positionTable)
	position := func(s loader.Sym, file bool) positionTable {
		if s == 0 {
			return positionTable{}
		}
		key := positionKey{s, file}
		if p, ok := positions[key]; ok {
			return p
		}
		var limit int64
		if file {
			limit = fileBytes
		}
		data, base := compactPCPosition(ldr.Data(s), limit)
		p := positionTable{offset: table.add(data), base: base}
		positions[key] = p
		return p
	}

	quantum := int(ctxt.Arch.MinLC)
	var scratch []loader.Sym
	for i, s := range funcs {
		p := &pcs.funcs[i]
		p.start, p.end = len(pcs.extra), len(pcs.extra)
		fi := ldr.FuncInfo(s)
		if !fi.Valid() {
			continue
		}
		fi.Preload()
		pcsp, pcfile, pcline, pcinline, pcdata := ldr.PcdataAuxs(s, scratch)
		p.pcsp = rawOffset(pcsp)
		file := position(pcfile, ldr.SymUnit(s) != nil)
		p.pcfile, p.fileBase = file.offset, file.base
		p.pcline = position(pcline, false).offset

		if fi.NumInlTree() > 0 {
			for len(pcdata) <= abi.PCDATA_InlTreeIndex {
				pcdata = append(pcdata, 0)
			}
			pcdata[abi.PCDATA_InlTreeIndex] = pcinline
		}
		textSize := ldr.SymSize(s)
		for _, sym := range pcdata {
			var off uint32
			if sym != 0 && !defaultPCData(ldr.Data(sym), quantum, textSize) {
				off = rawOffset(sym)
			}
			pcs.extra = append(pcs.extra, off)
		}
		for len(pcs.extra) > p.start && pcs.extra[len(pcs.extra)-1] == 0 {
			pcs.extra = pcs.extra[:len(pcs.extra)-1]
		}
		p.end = len(pcs.extra)
		scratch = pcdata
	}
	table.heads = nil
	return pcs, table
}

// defaultPCData reports whether an optional PCDATA stream is -1 over the
// entire function. The runtime returns -1 for an absent stream. The only
// PCDATA consumer of a run's start PC uses it for restartable unsafe points.
func defaultPCData(data []byte, quantum int, textSize int64) bool {
	if len(data) < 3 || data[0] != 0 || data[len(data)-1] != 0 {
		return false
	}
	span, n := binary.Uvarint(data[1:])
	return n == len(data)-2 && span > 0 && span <= math.MaxUint32/uint64(quantum) &&
		span*uint64(quantum) >= uint64(textSize)
}

// compactPCPosition preserves the value at every PC. For pcfile, the returned
// base is added to the function's cutab offset, so it still uses the same
// shared compilation-unit file entries with smaller file indices.
func compactPCPosition(data []byte, fileBytes int64) ([]byte, uint32) {
	if len(data) < 3 {
		return data, 0
	}
	// Work in units of PCQuantum. The encoder writes the same units.
	it := obj.NewPCIter(1)
	firstFile := int32(math.MaxInt32)
	rewrite := false
	for it.Init(data); !it.Done; it.Next() {
		if it.PC == it.NextPC {
			rewrite = true
			continue
		}
		if fileBytes == 0 || it.Value == -1 {
			continue
		}
		// Preserve lookup behavior for out-of-range file indices.
		if it.Value < 0 || int64(it.Value) >= fileBytes {
			fileBytes = 0
		}
		firstFile = min(firstFile, it.Value)
	}
	if fileBytes == 0 || firstFile == math.MaxInt32 {
		firstFile = 0
	}
	if !rewrite && firstFile == 0 {
		return data, 0
	}

	out := make([]byte, 0, len(data))
	prev, value := int32(-1), int32(-1)
	var start, end uint32
	appendRun := func() {
		out = binary.AppendVarint(out, int64(value-prev))
		out = binary.AppendUvarint(out, uint64(end-start))
		prev, start = value, end
	}
	for it.Init(data); !it.Done; it.Next() {
		if it.PC == it.NextPC {
			continue
		}
		next := it.Value
		if next >= 0 {
			next -= firstFile
		}
		if end != 0 && value != next {
			appendRun()
		}
		value, end = next, it.NextPC
	}
	if end == 0 {
		return data, 0
	}
	appendRun()
	return append(out, 0), uint32(firstFile)
}
