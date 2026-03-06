package zmodel

import (
	"context"
	"time"
)

type UpdateFuncItem struct {
	Do       func(ctx context.Context, nowTs int64)
	Interval time.Duration
	LastTime int64
	Name     string
}

func NewUpdateFuncItem(name string, interval time.Duration, f func(ctx context.Context, nowTs int64)) *UpdateFuncItem {
	return &UpdateFuncItem{
		Do:       f,
		Interval: interval,
		Name:     name,
	}
}
