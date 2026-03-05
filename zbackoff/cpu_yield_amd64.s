TEXT ·cpuYield(SB), $0-4
    MOVL cycles+0(FP), AX
    TESTL AX, AX
    JZ done
loop:
    PAUSE
    DECL AX
    JNZ loop
done:
    RET
