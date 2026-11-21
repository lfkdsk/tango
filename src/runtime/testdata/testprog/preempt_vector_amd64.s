// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

#include "textflag.h"

// Helpers for AsyncPreemptVector. vecFill leaves Y0-Y14 loaded on return
// (no VZEROUPPER), and vecCheck compares all 256 bits of each of them with
// the same patterns. Go code between the two calls runs with live upper
// vector state, as vectorized compiled code does.

// func vecHasAVX2() bool
TEXT ·vecHasAVX2(SB), NOSPLIT, $0-1
	MOVL	$1, AX
	XORL	CX, CX
	CPUID
	ANDL	$0x18000000, CX // OSXSAVE and AVX
	CMPL	CX, $0x18000000
	JNE	no
	XORL	CX, CX
	XGETBV
	ANDL	$6, AX // XMM and YMM state enabled by the OS
	CMPL	AX, $6
	JNE	no
	MOVL	$7, AX
	XORL	CX, CX
	CPUID
	TESTL	$0x20, BX // AVX2
	JZ	no
	MOVB	$1, ret+0(FP)
	RET
no:
	MOVB	$0, ret+0(FP)
	RET

// func vecFill(pat *[15][4]uint64)
TEXT ·vecFill(SB), NOSPLIT, $0-8
	MOVQ	pat+0(FP), AX
	VMOVDQU	0(AX), Y0
	VMOVDQU	32(AX), Y1
	VMOVDQU	64(AX), Y2
	VMOVDQU	96(AX), Y3
	VMOVDQU	128(AX), Y4
	VMOVDQU	160(AX), Y5
	VMOVDQU	192(AX), Y6
	VMOVDQU	224(AX), Y7
	VMOVDQU	256(AX), Y8
	VMOVDQU	288(AX), Y9
	VMOVDQU	320(AX), Y10
	VMOVDQU	352(AX), Y11
	VMOVDQU	384(AX), Y12
	VMOVDQU	416(AX), Y13
	VMOVDQU	448(AX), Y14
	RET

// func vecCheck(pat *[15][4]uint64) uint32
// The result has bit i set if Yi differs from pat[i].
TEXT ·vecCheck(SB), NOSPLIT, $0-12
	MOVQ	pat+0(FP), AX
	XORL	DX, DX
	VMOVDQU	0(AX), Y15
	VPCMPEQB	Y0, Y15, Y15
	VPMOVMSKB	Y15, CX
	CMPL	CX, $0xffffffff
	JEQ	ok0
	ORL	$1, DX
ok0:
	VMOVDQU	32(AX), Y15
	VPCMPEQB	Y1, Y15, Y15
	VPMOVMSKB	Y15, CX
	CMPL	CX, $0xffffffff
	JEQ	ok1
	ORL	$2, DX
ok1:
	VMOVDQU	64(AX), Y15
	VPCMPEQB	Y2, Y15, Y15
	VPMOVMSKB	Y15, CX
	CMPL	CX, $0xffffffff
	JEQ	ok2
	ORL	$4, DX
ok2:
	VMOVDQU	96(AX), Y15
	VPCMPEQB	Y3, Y15, Y15
	VPMOVMSKB	Y15, CX
	CMPL	CX, $0xffffffff
	JEQ	ok3
	ORL	$8, DX
ok3:
	VMOVDQU	128(AX), Y15
	VPCMPEQB	Y4, Y15, Y15
	VPMOVMSKB	Y15, CX
	CMPL	CX, $0xffffffff
	JEQ	ok4
	ORL	$16, DX
ok4:
	VMOVDQU	160(AX), Y15
	VPCMPEQB	Y5, Y15, Y15
	VPMOVMSKB	Y15, CX
	CMPL	CX, $0xffffffff
	JEQ	ok5
	ORL	$32, DX
ok5:
	VMOVDQU	192(AX), Y15
	VPCMPEQB	Y6, Y15, Y15
	VPMOVMSKB	Y15, CX
	CMPL	CX, $0xffffffff
	JEQ	ok6
	ORL	$64, DX
ok6:
	VMOVDQU	224(AX), Y15
	VPCMPEQB	Y7, Y15, Y15
	VPMOVMSKB	Y15, CX
	CMPL	CX, $0xffffffff
	JEQ	ok7
	ORL	$128, DX
ok7:
	VMOVDQU	256(AX), Y15
	VPCMPEQB	Y8, Y15, Y15
	VPMOVMSKB	Y15, CX
	CMPL	CX, $0xffffffff
	JEQ	ok8
	ORL	$256, DX
ok8:
	VMOVDQU	288(AX), Y15
	VPCMPEQB	Y9, Y15, Y15
	VPMOVMSKB	Y15, CX
	CMPL	CX, $0xffffffff
	JEQ	ok9
	ORL	$512, DX
ok9:
	VMOVDQU	320(AX), Y15
	VPCMPEQB	Y10, Y15, Y15
	VPMOVMSKB	Y15, CX
	CMPL	CX, $0xffffffff
	JEQ	ok10
	ORL	$1024, DX
ok10:
	VMOVDQU	352(AX), Y15
	VPCMPEQB	Y11, Y15, Y15
	VPMOVMSKB	Y15, CX
	CMPL	CX, $0xffffffff
	JEQ	ok11
	ORL	$2048, DX
ok11:
	VMOVDQU	384(AX), Y15
	VPCMPEQB	Y12, Y15, Y15
	VPMOVMSKB	Y15, CX
	CMPL	CX, $0xffffffff
	JEQ	ok12
	ORL	$4096, DX
ok12:
	VMOVDQU	416(AX), Y15
	VPCMPEQB	Y13, Y15, Y15
	VPMOVMSKB	Y15, CX
	CMPL	CX, $0xffffffff
	JEQ	ok13
	ORL	$8192, DX
ok13:
	VMOVDQU	448(AX), Y15
	VPCMPEQB	Y14, Y15, Y15
	VPMOVMSKB	Y15, CX
	CMPL	CX, $0xffffffff
	JEQ	ok14
	ORL	$16384, DX
ok14:
	VZEROUPPER
	MOVL	DX, ret+8(FP)
	RET
