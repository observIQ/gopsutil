// SPDX-License-Identifier: BSD-3-Clause
//go:build aix

#include "textflag.h"

TEXT ·syscall6(SB),NOSPLIT,$0-88
	JMP	syscall·syscall6(SB)
