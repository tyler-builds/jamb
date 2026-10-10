#include "textflag.h"

// elementFromPointStub is the vtable entry of IRawElementProviderFragmentRoot::
// ElementProviderFromPoint. A call to it passes this in CX, x in XMM1, y in
// XMM2 and the result pointer in R9. The callback reads the integer registers
// CX, DX, R8 and R9, so the stub moves the bits of x and y into DX and R8 and
// jumps to the callback, which then sees the arguments in its usual places.
TEXT ·elementFromPointStub(SB),NOSPLIT|NOFRAME,$0-0
	MOVQ	X1, DX
	MOVQ	X2, R8
	MOVQ	·elementFromPointCB(SB), AX
	JMP	AX

// func elementFromPointStubAddr() uintptr
TEXT ·elementFromPointStubAddr(SB),NOSPLIT,$0-8
	LEAQ	·elementFromPointStub(SB), AX
	MOVQ	AX, ret+0(FP)
	RET
