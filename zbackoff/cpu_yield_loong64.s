TEXT ·cpuYield(SB), $0-4
    MOVW cycles+0(FP), R4   // 加载32位参数，高32位清零
    BEQ R4, done
loop:
    YIELD
    ADDV $-1, R4
    BNE loop
done:
    RET
