// Copyright 2025 ByteDance Ltd. and/or its affiliates. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build goexperiment.tangollvm && linux && amd64 && cgo && !compiler_bootstrap

package llvm

import (
	"cmd/internal/obj"
	"cmd/internal/objabi"
	"encoding/binary"
	"fmt"
	"internal/abi"
	"iter"
	"log"
	"maps"
	"strings"
)

// Common logic of patch ELF or MachO to goobj
// Host is ELF or MachO
// Target is the target object file, e.g., goobj,
// or the internal linker state of goobj.
type TargetSym interface {
	SymId() int64
}

// Except the HostName, all other fields
// can be directly used in the target.
type TargetSymbol struct {
	HostName string
	Size     uint64
	P        []byte
	R        []TargetReloc
}

type TargetReloc struct {
	Off      int32
	Siz      uint8
	Type     objabi.RelocType
	Add      int64
	HostName string
	Sym      TargetSym
}

type Patcher interface {
	SymbolNames() iter.Seq[string]
	Symbol(name string) *TargetSymbol
	EnsureSymbol(name string) *TargetSymbol
	Auxs(name string) []*TargetSymbol
	ApplyPatches()
}

type PatchContext interface {
	// iterate each text symbol
	ForEachTextSym(f func(pctxt PatchContext, tsym TargetSym, tsymbol *TargetSymbol))

	PatchSymbol(tsym TargetSym, symbol *TargetSymbol)
	PatchFuncInfo(tsym TargetSym, fisymbol *TargetSymbol)
	// Keep the signature the same as other place
	PatchPCData(tsym TargetSym, pcsp, pcfile, pcline, pcinline *TargetSymbol, pcdata []*TargetSymbol)
	PatchFuncData(fsym TargetSym, fdid int, fdsymbol *TargetSymbol)
	Init(p Patcher)

	SymName(sym TargetSym) string
	SymPkg(sym TargetSym) string
	SymSize(sym TargetSym) int64
	SymAttrNoSplit(sym TargetSym) bool

	// LookupSym will fail if the symbol does not exist
	LookupSym(name string) TargetSym
	// FindSym will return nil if the symbol does not exist
	FindSym(name string) TargetSym
	// LookupTypeSym will fail if the symbol does not exist
	LookupTypeSym(tyid string) TargetSym

	CreateConstDataSymbol(data []byte, size int64, align int32) TargetSym
}

func parseItabSymbol(s string) (t1, t2 string) {
	twotypes := s[len(ItabPrefix):]
	depth := 0
	for i := 0; i < len(twotypes); i++ {
		b := twotypes[i]
		switch b {
		case '[':
			depth++
		case ']':
			depth--
		case ',':
			if depth == 0 {
				return twotypes[:i], twotypes[i+1:]
			}
		}
	}
	panic("unreachable: cannot parse itab symbol " + s)
}

// sync with ir.Pkgs.Itab.Prefix
const ItabPrefix = "go:itab."

type patcher struct {
	pctxt   PatchContext
	symbols map[string]*TargetSymbol
	verbose int
}

func collectMarkerRelocation(pctxt PatchContext, s *TargetSymbol) {
	var added []TargetReloc
	for _, r := range s.R {
		if r.Sym == nil {
			continue
		}
		// Look for itab
		// TODO: we may need to handle other R_USEXXX marker relocations
		name := pctxt.SymName(r.Sym)
		if strings.HasPrefix(name, ItabPrefix) {
			// See reflectdata.ITabAddr for format details
			srcTypeId, _ := parseItabSymbol(name)
			srcTypeSym := pctxt.LookupTypeSym(srcTypeId)
			// assume itabs are not too many
			found := false
			// we do not iterate over lsym.R
			for i := range added {
				rr := &added[i]
				if rr.Sym == srcTypeSym {
					found = true
					break
				}
			}
			if !found {
				added = append(added, TargetReloc{
					Type: objabi.R_USEIFACE,
					Sym:  srcTypeSym,
				})
			}
		}
	}
	if len(added) > 0 {
		// offset 0 should be first
		s.R = append(added, s.R...)
	}
}

func extractPatchSymbols(ctxt PatchContext, data []byte) map[string]*TargetSymbol {
	c1, c2, c3, c4 := data[0], data[1], data[2], data[3]
	magic := uint32(c1)<<24 | uint32(c2)<<16 | uint32(c3)<<8 | uint32(c4)
	if magic == 0x7f454c46 { // \x7F E L F
		return extractSymbolsFromELF(ctxt, data)
	} else if magic&^1 == 0xfeedface || magic&^0x01000000 == 0xcefaedfe {
		return extractSymbolsFromMachO(ctxt, data)
	}
	log.Fatalf("unsupported file format: magic=%x", magic)
	return nil
}

func (p *patcher) SymbolNames() iter.Seq[string] { return maps.Keys(p.symbols) }

func (p *patcher) Symbol(name string) *TargetSymbol {
	if sym, ok := p.symbols[name]; ok {
		return sym
	}
	return nil
}

func (p *patcher) EnsureSymbol(name string) *TargetSymbol {
	if sym, ok := p.symbols[name]; ok {
		return sym
	}
	for k := range p.symbols {
		fmt.Println(k)
	}
	panic("cannot find patch symbol " + name)
}

func (p *patcher) Auxs(hostname string) []*TargetSymbol {
	if _, ok := p.symbols[hostname]; !ok {
		panic("oops " + hostname + " not found")
	}
	funcinfosym := p.EnsureSymbol(hostname + "..funcinfo")
	stkobjsym := p.Symbol(hostname + "..stackobjects")
	argsmapsym := p.Symbol(hostname + "..argspointermaps")
	localsmapsym := p.Symbol(hostname + "..localspointermaps")
	pcspsym := p.Symbol(hostname + "..pcsp")
	pcfilesym := p.Symbol(hostname + "..pcfile")
	pclinesym := p.Symbol(hostname + "..pcline")
	pcinlinesym := p.Symbol(hostname + "..pcinline")
	pcstackmapindexsym := p.Symbol(hostname + "..pcstackmapindex")
	pcunsafesym := p.Symbol(hostname + "..pcunsafepoint")
	/*
		PCDATA_UnsafePoint   = 0
		PCDATA_StackMapIndex = 1
		PCDATA_InlTreeIndex  = 2
		PCDATA_ArgLiveIndex  = 3
	*/
	return []*TargetSymbol{
		funcinfosym,
		localsmapsym,
		argsmapsym,
		stkobjsym,
		pcspsym,
		pcfilesym,
		pclinesym,
		pcunsafesym,
		pcstackmapindexsym,
		pcinlinesym,
		p.Symbol(hostname + "..pcargliveindex"),
		p.Symbol(hostname + "..argliveinfo"),
		p.Symbol(hostname + "..arginfo"),
	}
}

// packArgLivePCData converts resolved (PC, bitmap offset) pairs to Go PCDATA.
// Multiple labels may resolve to one PC, for example at an empty machine block.
// In that case only the last state at that PC applies. The final pair gives
// the end of the function and has no state.
func packArgLivePCData(raw *TargetSymbol) *TargetSymbol {
	if len(raw.P) < 16 || len(raw.P)%8 != 0 {
		panic("invalid LLVM argument liveness PC table")
	}
	var data []byte
	prev := int64(-1)
	var span uint64
	var delta int64
	for off := 0; off+8 < len(raw.P); off += 8 {
		pc := binary.LittleEndian.Uint32(raw.P[off:])
		next := binary.LittleEndian.Uint32(raw.P[off+8:])
		if pc == next {
			continue
		}
		if pc > next {
			panic("invalid LLVM argument liveness PC range")
		}
		value := int64(binary.LittleEndian.Uint32(raw.P[off+4:]))
		if value != prev {
			if span != 0 {
				data = binary.AppendVarint(data, delta)
				data = binary.AppendUvarint(data, span)
			}
			delta = value - prev
			prev = value
			span = 0
		}
		span += uint64(next - pc)
	}
	if span != 0 {
		data = binary.AppendVarint(data, delta)
		data = binary.AppendUvarint(data, span)
	}
	data = append(data, 0)
	return &TargetSymbol{HostName: raw.HostName, P: data, Size: uint64(len(data))}
}

func (p *patcher) patchText(pctxt PatchContext, tsym TargetSym, tsymbol *TargetSymbol) {
	if tsymbol == nil {
		return
	}
	if tsym == nil {
		panic("nil target symbol")
	}
	hostname := tsymbol.HostName
	attrbutessym := p.EnsureSymbol(hostname + "..attributes")
	if len(attrbutessym.P) != 8 {
		panic("must be int64")
	}
	// TODO: little endian
	attributes := obj.Attribute(binary.LittleEndian.Uint64(attrbutessym.P))
	if attributes.NoSplit() != pctxt.SymAttrNoSplit(tsym) {
		return
	}

	collectMarkerRelocation(pctxt, tsymbol)
	pctxt.PatchSymbol(tsym, tsymbol)

	funcinfosym := p.EnsureSymbol(hostname + "..funcinfo")
	pctxt.PatchFuncInfo(tsym, funcinfosym)

	/*
		FUNCDATA_ArgsPointerMaps    = 0
		FUNCDATA_LocalsPointerMaps  = 1
		FUNCDATA_StackObjects       = 2
	*/
	stkobjsym := p.Symbol(hostname + "..stackobjects")
	argsmapsym := p.Symbol(hostname + "..argspointermaps")
	localsmapsym := p.Symbol(hostname + "..localspointermaps")
	pctxt.PatchFuncData(tsym, abi.FUNCDATA_StackObjects, stkobjsym)
	pctxt.PatchFuncData(tsym, abi.FUNCDATA_ArgsPointerMaps, argsmapsym)
	pctxt.PatchFuncData(tsym, abi.FUNCDATA_LocalsPointerMaps, localsmapsym)
	if deferInfo := p.Symbol(hostname + "..opendefer"); deferInfo != nil {
		pctxt.PatchFuncData(tsym, abi.FUNCDATA_OpenCodedDeferInfo, deferInfo)
	}

	pcspsym := p.Symbol(hostname + "..pcsp")
	pcfilesym := p.Symbol(hostname + "..pcfile")
	pclinesym := p.Symbol(hostname + "..pcline")
	pcinlinesym := p.Symbol(hostname + "..pcinline")
	pcstackmapindexsym := p.Symbol(hostname + "..pcstackmapindex")
	arginfo := p.Symbol(hostname + "..arginfo1")
	arglive := p.Symbol(hostname + "..argliveinfo")
	pcargliveindexraw := p.Symbol(hostname + "..pcargliveindex")
	var pcargliveindexsym *TargetSymbol
	if arglive != nil && pcargliveindexraw != nil {
		pcargliveindexsym = packArgLivePCData(pcargliveindexraw)
	} else if arglive != nil || pcargliveindexraw != nil {
		panic("incomplete LLVM argument liveness metadata")
	}
	pctxt.PatchFuncData(tsym, abi.FUNCDATA_ArgInfo, arginfo)
	pctxt.PatchFuncData(tsym, abi.FUNCDATA_ArgLiveInfo, arglive)

	pcunsafesym := p.Symbol(hostname + "..pcunsafepoint")
	/*
		PCDATA_UnsafePoint   = 0
		PCDATA_StackMapIndex = 1
		PCDATA_InlTreeIndex  = 2
		PCDATA_ArgLiveIndex  = 3
	*/
	pcdatasyms := []*TargetSymbol{
		pcunsafesym,
		pcstackmapindexsym,
		pcinlinesym,
		pcargliveindexsym,
	}
	pctxt.PatchPCData(tsym, pcspsym, pcfilesym, pclinesym, pcinlinesym, pcdatasyms)
}

// A Go symbol may have a version/ABI.
// But LLVM does not support it. We have to mangle Go symbols for LLVM.
// Luckily, only symbols for functions (and in TEXT) need ABI info.
// All data are ABI0. So we only need to encode function names.
// mangleABIName in symtab.go has rules for how to mangle names in the final binary.
// We have no need to follow those rules because LLVM's output is only used for
// temporary storage. All data in LLVM's output will be patched into the object
// file (LSym indeed) generated by the gc compiler.
//
// Here we make things simple.
// 1. LLVM never introduces a new Go symbol that needs ABI info.
// 2. LLVM may generate a new static symbol that is not available in the gc compiler.
//
//  1. Symbols in TEXT may have an ABI suffix
//     a. "..abi0" for ABI0 (additional dot in case the function's name is `abi0')
//     b. nothing for ABIInternal
//  2. We have no need to deal with ABI for symbols in other sections.
//
// How to recover a symbol and its type
// 1. If the symbol has a suffix "..abi0", it must be an existing text ABI0 symbol.
// 2. Otherwise, if the symbol is only available in either ABI0 or ABIInternal's hash but not both,
// it must be
// 3. If the symbol is available in both ABI, then it must be a text ABIInternal symbol.
// 4. If the symbol is not available, it must be a static symbol.
func EncodeFuncNameAndABI(name string, abi obj.ABI) string {
	if abi == obj.ABI0 {
		return name + "..abi0"
	}
	return name
}

func DecodeFuncNameAndABI(name string) (string, obj.ABI) {
	if strings.HasSuffix(name, "..abi0") {
		return strings.TrimSuffix(name, "..abi0"), obj.ABI0
	}
	return name, obj.ABIInternal
}

func NewPatcher(pctxt PatchContext, b []byte, verbose int) Patcher {
	p := &patcher{
		pctxt:   pctxt,
		symbols: extractPatchSymbols(pctxt, b),
		verbose: verbose,
	}
	pctxt.Init(p)
	return p
}

func (p *patcher) ApplyPatches() {
	if p.verbose >= 2 {
		for l, s := range p.symbols {
			fmt.Printf("AuxSymbol: %s, size=%d\n", l, s.Size)
			if s.Size != uint64(len(s.P)) {
				panic("oops")
			}
			for i, r := range s.R {
				name := ""
				size := int64(0)
				if r.Sym != nil {
					name = p.pctxt.SymName(r.Sym)
					size = p.pctxt.SymSize(r.Sym)
				}
				fmt.Printf("\t%d: reloc@%d: %s, size=%d\n", i, r.Off, name, size)
			}
		}
	}
	p.pctxt.ForEachTextSym(p.patchText)
}

func Patch(pctxt PatchContext, b []byte, verbose int) {
	p := NewPatcher(pctxt, b, verbose)
	p.ApplyPatches()
}
