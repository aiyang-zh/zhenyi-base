//go:build !amd64 && !arm64

package zserialize

import (
	gojson "github.com/goccy/go-json"
)

func UnmarshalJson(body []byte, data interface{}) error {
	return gojson.Unmarshal(body, data)
}

func MarshalJson(v interface{}) ([]byte, error) {
	return gojson.Marshal(v)
}
