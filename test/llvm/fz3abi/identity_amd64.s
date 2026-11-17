// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

#include "textflag.h"

TEXT ·identity(SB), NOSPLIT, $0-16
	MOVQ x+0(FP), AX
	MOVQ AX, ret+8(FP)
	RET

TEXT ·asmPC(SB), NOSPLIT, $0-8
	LEAQ ·identity(SB), AX
	MOVQ AX, ret+0(FP)
	RET
