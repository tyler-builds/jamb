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

// setScrollPercentStub is the vtable entry of IScrollProvider::SetScrollPercent,
// which takes two doubles like ElementProviderFromPoint. It moves them from XMM1
// and XMM2 into DX and R8, as elementFromPointStub does, and jumps to the
// callback that setScrollPercentCB holds.
TEXT ·setScrollPercentStub(SB),NOSPLIT|NOFRAME,$0-0
	MOVQ	X1, DX
	MOVQ	X2, R8
	MOVQ	·setScrollPercentCB(SB), AX
	JMP	AX

// func setScrollPercentStubAddr() uintptr
TEXT ·setScrollPercentStubAddr(SB),NOSPLIT,$0-8
	LEAQ	·setScrollPercentStub(SB), AX
	MOVQ	AX, ret+0(FP)
	RET

// setRangeValueStub is the vtable entry of IRangeValueProvider::SetValue, which
// takes one double. The double is in XMM1, so the stub moves it into DX, the
// integer register the callback reads, and jumps to the callback that
// setRangeValueCB holds.
TEXT ·setRangeValueStub(SB),NOSPLIT|NOFRAME,$0-0
	MOVQ	X1, DX
	MOVQ	·setRangeValueCB(SB), AX
	JMP	AX

// func setRangeValueStubAddr() uintptr
TEXT ·setRangeValueStubAddr(SB),NOSPLIT,$0-8
	LEAQ	·setRangeValueStub(SB), AX
	MOVQ	AX, ret+0(FP)
	RET
