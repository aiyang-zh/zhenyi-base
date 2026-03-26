//go:build sonic && (amd64 || arm64)

package zserialize

import (
	"github.com/bytedance/sonic"
)

func UnmarshalJson(body []byte, data interface{}) error {
	return sonic.ConfigFastest.Unmarshal(body, data)
}

func MarshalJson(v interface{}) ([]byte, error) {
	return sonic.ConfigFastest.Marshal(v)
}
