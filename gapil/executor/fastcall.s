
#include "go_asm.h"
#include "textflag.h"
#include "funcdata.h"

#define   get_tls(r)   MOVQ TLS, r
#define   g(r)   0(r)(TLS*1)

#ifdef GOOS_windows
#define RARG0 CX
#define RARG1 DX
#define RARG2 R8
#define RARG3 R9
#else
#define RARG0 DI
#define RARG1 SI
#define RARG2 DX
#define RARG3 CX
#define RARG4 R8
#define RARG5 R9
#endif

TEXT ·fastcallC(SB), 0, $0-0
 MOVQ   pfn+0(FP), AX
 MOVQ   ctx+8(FP), RARG0
 MOVQ   module+16(FP), RARG1
 MOVQ   cmds+24(FP), RARG2
 MOVQ   count+32(FP), RARG3
 MOVQ   results+40(FP), RARG4
 MOVQ   0(RARG0), R11 // load C stack
 MOVQ   SP, R12       // assign SP to R12 (preserved across C calls)
 MOVQ   SP, 0(RARG0)  // store Go stack
 MOVQ   R11, SP       // switch to C stack
 CALL   AX            // call pfn
 MOVQ   R12, SP       // restore Go stack
 RET

#define SWAP_STACKS(ctx)                         \
 MOVQ   0(ctx), R11 /* load target stack */      \
 MOVQ   SP, 0(ctx)  /* store current stack */    \
 MOVQ   R11, SP     /* switch to target stack */

#define SAVE_CALLEE_REG \
 PUSHQ  BX              \
 PUSHQ  BP              \
 PUSHQ  DI              \
 PUSHQ  SI              \
 PUSHQ  R12             \
 PUSHQ  R13             \
 PUSHQ  R14             \
 PUSHQ  R15

#define RESTORE_CALLEE_REG \
 POPQ   R15                \
 POPQ   R14                \
 POPQ   R13                \
 POPQ   R12                \
 POPQ   SI                 \
 POPQ   DI                 \
 POPQ   BP                 \
 POPQ   BX

#define PUSH_RESULT \
 PUSHQ  R12

#define POP_RESULT \
 POPQ   AX

#define PUSH_1_ARG \
 PUSHQ  RARG0

#define PUSH_2_ARGS \
 PUSHQ  RARG1       \
 PUSH_1_ARG

#define PUSH_3_ARGS \
 PUSHQ  RARG2       \
 PUSH_2_ARGS

#define PUSH_4_ARGS \
 PUSHQ  RARG3       \
 PUSH_3_ARGS

#define PUSH_5_ARGS \
 PUSHQ  RARG4       \
 PUSH_4_ARGS

#define POP_1_ARG \
 POPQ   R12

#define POP_2_ARGS \
 POP_1_ARG         \
 POPQ  R12

#define POP_3_ARGS \
 POP_2_ARGS        \
 POPQ  R12         \

#define POP_4_ARGS \
 POP_3_ARGS        \
 POPQ  R12         \

#define POP_5_ARGS \
 POP_4_ARGS        \
 POPQ  R12         \

TEXT ·applyReadsFC(SB), NOSPLIT, $0-0
 SWAP_STACKS(RARG0) // switch to go stack
 SAVE_CALLEE_REG
 PUSH_1_ARG
 CALL ·applyReads(SB)
 POP_1_ARG
 RESTORE_CALLEE_REG
 SWAP_STACKS(RARG0) // switch to C stack
 RET

TEXT ·applyWritesFC(SB), NOSPLIT, $0-0
 SWAP_STACKS(RARG0) // switch to go stack
 SAVE_CALLEE_REG
 PUSH_1_ARG
 CALL ·applyWrites(SB)
 POP_1_ARG
 RESTORE_CALLEE_REG
 SWAP_STACKS(RARG0) // switch to C stack
 RET

TEXT ·copySliceFC(SB), NOSPLIT, $0-0
 SWAP_STACKS(RARG0) // switch to go stack
 SAVE_CALLEE_REG
 PUSH_3_ARGS
 CALL ·copySlice(SB)
 POP_3_ARGS
 RESTORE_CALLEE_REG
 SWAP_STACKS(RARG0) // switch to C stack
 RET

TEXT ·cstringToSliceFC(SB), NOSPLIT, $0-0
 SWAP_STACKS(RARG0) // switch to go stack
 SAVE_CALLEE_REG
 PUSH_3_ARGS
 CALL ·cstringToSlice(SB)
 POP_3_ARGS
 RESTORE_CALLEE_REG
 SWAP_STACKS(RARG0) // switch to C stack
 RET

TEXT ·makePoolFC(SB), NOSPLIT, $0-0
 SWAP_STACKS(RARG0) // switch to go stack
 SAVE_CALLEE_REG
 PUSH_RESULT
 PUSH_2_ARGS
 CALL ·makePool(SB)
 POP_2_ARGS
 POP_RESULT
 RESTORE_CALLEE_REG
 SWAP_STACKS(RARG0) // switch to C stack
 RET

TEXT ·poolReferenceFC(SB), NOSPLIT, $0-0
 SWAP_STACKS(RARG0) // switch to go stack
 SAVE_CALLEE_REG
 PUSH_2_ARGS
 CALL ·poolReference(SB)
 POP_2_ARGS
 RESTORE_CALLEE_REG
 SWAP_STACKS(RARG0) // switch to C stack
 RET

TEXT ·poolReleaseFC(SB), NOSPLIT, $0-0
 SWAP_STACKS(RARG0) // switch to go stack
 SAVE_CALLEE_REG
 PUSH_2_ARGS
 CALL ·poolRelease(SB)
 POP_2_ARGS
 RESTORE_CALLEE_REG
 SWAP_STACKS(RARG0) // switch to C stack
 RET

TEXT ·resolvePoolDataFC(SB), NOSPLIT, $0-0
 SWAP_STACKS(RARG0) // switch to go stack
 SAVE_CALLEE_REG
 PUSH_RESULT
 PUSH_5_ARGS
 CALL ·resolvePoolData(SB)
 POP_5_ARGS
 POP_RESULT
 RESTORE_CALLEE_REG
 SWAP_STACKS(RARG0) // switch to C stack
 RET

TEXT ·storeInDatabaseFC(SB), NOSPLIT, $0-0
 SWAP_STACKS(RARG0) // switch to go stack
 SAVE_CALLEE_REG
 PUSH_4_ARGS
 CALL ·storeInDatabase(SB)
 POP_4_ARGS
 RESTORE_CALLEE_REG
 SWAP_STACKS(RARG0) // switch to C stack
 RET

TEXT ·callExternFC(SB), NOSPLIT, $0-0
 SWAP_STACKS(RARG0) // switch to go stack
 SAVE_CALLEE_REG
 PUSH_4_ARGS
 CALL ·callExtern(SB)
 POP_4_ARGS
 RESTORE_CALLEE_REG
 SWAP_STACKS(RARG0) // switch to C stack
 RET
