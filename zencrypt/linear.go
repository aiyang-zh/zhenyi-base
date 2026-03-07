package zencrypt

import (
	"math/big"
	"math/rand"
	"time"
)

// 线性加密
// 加密公式 new_id = (old_id * a + b) % m  注：a 和 m 互为质数  (b 的值为随机值 值不同 结果不同 但是不会影响结果分布)

type Linear struct {
	A   *big.Int
	B   *big.Int
	Mod *big.Int
}

func NewLinear(a, b, mod int64) Linear {
	return Linear{
		A:   big.NewInt(a),
		B:   big.NewInt(b),
		Mod: big.NewInt(mod),
	}
}

func (l Linear) Encrypt(id int64) int64 {
	var _id = big.NewInt(id)
	var genId = new(big.Int).Mul(_id, l.A)
	genId.Add(genId, l.B)
	genId.Mod(genId, l.Mod)
	return genId.Int64()
}

// Decrypt
// 在线性映射中，假设我们知道 a、b、m以及new_id，并希望计算原始的 id。
// new_id = (a * id + b) % m 这是一个模运算，当且仅当 a 和 m 互质时，才可能对其进行逆操作找到原始的 id。
// 假设 a 和 m 是互质的，那么 a 逆元存在，记作 a’，满足 (a * a’) % m = 1。因此，就可以计算出 id = (a’ * (new_id - b)) % m。
// 计算 a’ 的方法是使用扩展欧几里得算法。
func (l Linear) Decrypt(id int64) int64 {
	_, invA, _ := l.parseId(l.A.Int64(), l.Mod.Int64())
	id_ := ((id - l.B.Int64()) * invA) % l.Mod.Int64()
	if id_ < 0 {
		id_ += l.Mod.Int64()
	}
	return id_
}

func (l Linear) parseId(a, b int64) (int64, int64, int64) {
	if a == 0 {
		return b, 0, 1
	} else {
		g, x, y := l.parseId(b%a, a)
		return g, y - (b/a)*x, x
	}
}

// CalculateA 线性混淆A值确定
func (l Linear) CalculateA() int64 {
	return l.findCoprime(l.Mod.Int64())
}

// 寻找互质数
func (l Linear) findCoprime(n int64) int64 {
	rand_ := rand.New(rand.NewSource(time.Now().UnixNano()))
	for {
		a := rand_.Int63n(n-1) + 1
		if l.gcd(a, n) == 1 {
			return a
		}
	}
}

// 求最大公约数
func (l Linear) gcd(a, b int64) int64 {
	for b != 0 {
		a, b = b, a%b
	}
	return a
}
