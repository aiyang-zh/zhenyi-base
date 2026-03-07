//go:build !amd64 && !arm64

package zserialize

func UnmarshalJson(body []byte, data interface{}) error {
	return json.Unmarshal(body, data)
}

func MarshalJson(v interface{}) ([]byte, error) {
	return json.Marshal(v)
}
