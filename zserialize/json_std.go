//go:build !sonic

package zserialize

import "encoding/json"

// 默认 JSON 实现：使用标准库 encoding/json。
// 如需启用 sonic（仅 amd64/arm64），请在编译时添加：-tags sonic
func UnmarshalJson(body []byte, data interface{}) error {
	return json.Unmarshal(body, data)
}

func MarshalJson(v interface{}) ([]byte, error) {
	return json.Marshal(v)
}
