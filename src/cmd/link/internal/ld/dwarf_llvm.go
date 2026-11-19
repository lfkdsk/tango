// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ld

import (
	"cmd/internal/dwarf"
	"cmd/internal/obj"
	"cmd/internal/objabi"
	"cmd/internal/src"
	"cmd/link/internal/loader"
	"cmd/link/internal/sym"
	"strings"
)

// The DWARF passes share these replacement auxiliaries. The loader's
// Go-object aux table keeps the original reader for its runtime metadata.
type llvmDwarfAux struct {
	info, ranges, lines loader.Sym
}

func (d *dwctxt) funcDwarfAux(fn loader.Sym) (info, loc, ranges, lines loader.Sym) {
	if aux, ok := d.linkctxt.llvmDwarf[fn]; ok {
		return aux.info, 0, aux.ranges, aux.lines
	}
	return d.ldr.GetFuncDwarfAuxSyms(fn)
}

// The compiler's native DIE describes the original Go machine code.
// Rebuild it from the LLVM tables after all compiler and LTO patching.
func (d *dwctxt) rebuildLLVMDWARF(fn loader.Sym, fi loader.FuncInfo) {
	ldr := d.ldr
	size := ldr.SymSize(fn)
	if size == 0 {
		return
	}
	oldInfo, _, _, _ := ldr.GetFuncDwarfAuxSyms(fn)
	name := ldr.SymName(fn)
	if oldInfo == 0 {
		// A linker-generated specialization has no Go compiler DIE.
		original, _, specialized := strings.Cut(name, ",,,")
		if !specialized {
			return
		}
		s := ldr.Lookup(original, ldr.SymVersion(fn))
		if s == 0 {
			return
		}
		oldInfo, _, _, _ = ldr.GetFuncDwarfAuxSyms(s)
		if oldInfo == 0 {
			return
		}
	}
	fi.Preload()
	var tmp [8]loader.Sym
	_, pcfile, pcline, pcinline, _ := ldr.PcdataAuxs(fn, tmp[:])
	lines, entryFile := d.llvmDWARFLines(fn, size, pcfile, pcline)
	ranges := ldr.MakeSymbolBuilder("")
	ranges.SetType(sym.SDWARFRANGE)
	info := ldr.MakeSymbolBuilder("")
	info.SetType(sym.SDWARFFCN)

	// Keep type uses from the Go compiler even though its local and
	// parameter locations refer to a different stack frame.
	relocs := ldr.Relocs(oldInfo)
	for i := 0; i < relocs.Count(); i++ {
		r := relocs.At(i)
		if r.Type() == objabi.R_USETYPE {
			nr, _ := info.AddRel(objabi.R_USETYPE)
			nr.SetSym(r.Sym())
		}
	}

	state := &dwarf.FnState{
		Name:          name,
		Info:          dwSym(info.Sym()),
		Ranges:        dwSym(ranges.Sym()),
		StartPC:       dwSym(fn),
		StartPos:      llvmDWARFPos(entryFile, fi.StartLine()),
		Size:          size,
		External:      !ldr.IsFileLocal(fn),
		InlCalls:      d.llvmDWARFInlines(fi, pcinline, size),
		UseBASEntries: d.linkctxt.Target.HeadType != objabi.Hdarwin,
	}
	if err := dwarf.PutDefaultFunc(d, state, false); err != nil {
		d.linkctxt.Errorf(fn, "LLVM DWARF: %v", err)
		return
	}
	if d.linkctxt.llvmDwarf == nil {
		d.linkctxt.llvmDwarf = make(map[loader.Sym]llvmDwarfAux)
	}
	d.linkctxt.llvmDwarf[fn] = llvmDwarfAux{info.Sym(), ranges.Sym(), lines}
}

func llvmDWARFPos(file, line int32) src.Pos {
	base := src.NewFileBase("", "")
	base.SetFileIndex(int(file))
	return src.MakePos(base, uint(max(line, 0)), 0)
}

func (d *dwctxt) llvmDWARFInlines(fi loader.FuncInfo, pcinline loader.Sym, size int64) dwarf.InlCalls {
	n := int(fi.NumInlTree())
	if n == 0 || pcinline == 0 {
		return dwarf.InlCalls{}
	}
	nodes := make([]loader.InlTreeNode, n)
	scopes := make([]dwarf.Scope, n)
	for i := range nodes {
		nodes[i] = fi.InlTree(i)
	}
	it := obj.NewPCIter(uint32(d.arch.MinLC))
	for it.Init(d.ldr.Data(pcinline)); !it.Done; it.Next() {
		if it.Value >= 0 && int(it.Value) < n {
			scopes[it.Value].AppendRange(dwarf.Range{
				Start: int64(it.PC),
				End:   min(int64(it.NextPC), size),
			})
		}
	}
	// Parent indices precede child indices in the emitted table. Fold in
	// children before emitting parents so nested ranges are contained.
	for i := n - 1; i >= 0; i-- {
		if parent := nodes[i].Parent; parent >= 0 {
			scopes[parent].UnifyRanges(&scopes[i])
		}
	}
	calls := dwarf.InlCalls{Calls: make([]dwarf.InlCall, n)}
	for i, node := range nodes {
		call := &calls.Calls[i]
		call.InlIndex = i
		call.Root = node.Parent < 0
		if len(scopes[i].Ranges) == 0 {
			continue
		}
		call.CallPos = llvmDWARFPos(int32(node.File), node.Line)
		call.AbsFunSym = d.llvmDWARFAbstract(node.Func)
		call.Ranges = scopes[i].Ranges
		if !call.Root {
			parent := &calls.Calls[node.Parent]
			parent.Children = append(parent.Children, i)
		}
	}
	return calls
}

func (d *dwctxt) llvmDWARFAbstract(fn loader.Sym) dwarf.Sym {
	if abs := d.llvmAbstract[fn]; abs != nil {
		return abs
	}
	ldr := d.ldr
	name := ldr.SymName(fn)
	abs := ldr.Lookup(dwarf.InfoPrefix+name+dwarf.AbstractFuncSuffix, 0)
	if abs == 0 || ldr.SymType(abs) != sym.SDWARFABSFCN {
		sb := ldr.MakeSymbolBuilder("")
		sb.SetType(sym.SDWARFABSFCN)
		abs = sb.Sym()
		fi := ldr.FuncInfo(fn)
		var line int32
		if fi.Valid() {
			line = fi.StartLine()
		}
		state := &dwarf.FnState{
			Name:     name,
			Absfn:    dwSym(abs),
			StartPos: llvmDWARFPos(0, line),
			External: !ldr.IsFileLocal(fn),
		}
		if err := dwarf.PutAbstractFunc(d, state); err != nil {
			d.linkctxt.Errorf(fn, "LLVM abstract DWARF: %v", err)
		}
	}
	if d.llvmAbstract == nil {
		d.llvmAbstract = make(map[loader.Sym]dwarf.Sym)
	}
	d.llvmAbstract[fn] = dwSym(abs)
	return dwSym(abs)
}

func (d *dwctxt) llvmDWARFLines(fn loader.Sym, size int64, pcfile, pcline loader.Sym) (loader.Sym, int32) {
	if pcfile == 0 || pcline == 0 {
		return 0, 0
	}
	lines := d.ldr.MakeSymbolBuilder("")
	lines.SetType(sym.SDWARFLINES)
	ls := dwSym(lines.Sym())
	lines.AddUint8(0)
	dwarf.Uleb128put(d, ls, 1+int64(d.PtrSize()))
	lines.AddUint8(dwarf.DW_LNE_set_address)
	d.AddAddress(ls, dwSym(fn), 0)

	file := obj.NewPCIter(uint32(d.arch.MinLC))
	line := obj.NewPCIter(uint32(d.arch.MinLC))
	file.Init(d.ldr.Data(pcfile))
	line.Init(d.ldr.Data(pcline))
	entryFile := max(file.Value, 0)
	prevFile, prevLine := int32(0), int32(1)
	var start, prevPC int64
	for !file.Done && !line.Done && start < size {
		end := int64(min(file.NextPC, line.NextPC))
		if end > start && file.Value >= 0 && (start == 0 || file.Value != prevFile || line.Value != prevLine) {
			if file.Value != prevFile {
				lines.AddUint8(dwarf.DW_LNS_set_file)
				dwarf.Uleb128put(d, ls, int64(file.Value)+1)
			}
			if start != prevPC {
				lines.AddUint8(dwarf.DW_LNS_advance_pc)
				dwarf.Uleb128put(d, ls, start-prevPC)
			}
			if line.Value != prevLine {
				lines.AddUint8(dwarf.DW_LNS_advance_line)
				dwarf.Sleb128put(d, ls, int64(line.Value)-int64(prevLine))
			}
			lines.AddUint8(dwarf.DW_LNS_copy)
			prevPC, prevFile, prevLine = start, file.Value, line.Value
		}
		if int64(file.NextPC) == end {
			file.Next()
		}
		if int64(line.NextPC) == end {
			line.Next()
		}
		start = end
	}
	lines.AddUint8(dwarf.DW_LNS_advance_pc)
	dwarf.Uleb128put(d, ls, size-prevPC)
	lines.AddBytes([]byte{0, 1, dwarf.DW_LNE_end_sequence})
	return lines.Sym(), entryFile
}
