TEXT ·cpuYield(SB), $0-4
    MOVW cycles+0(FP), R0   // 加载32位参数，高32位清零
    CBZ R0, done            // 如果为0，直接返回
loop:
    YIELD
    SUBS $1, R0
    BNE loop
done:
    RET
