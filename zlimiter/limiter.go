package zlimiter

import "golang.org/x/time/rate"

type ILimit interface {
	Allow() bool
}
type Limiter struct {
	Limit    int
	MaxLimit int
	limiter  *rate.Limiter
}

func NewLimiter(limit, maxLimit int) *Limiter {
	return &Limiter{
		Limit:    limit,
		MaxLimit: maxLimit,
		limiter:  rate.NewLimiter(rate.Limit(limit), maxLimit),
	}
}

func (l *Limiter) Allow() bool {
	return l.limiter.Allow()
}
