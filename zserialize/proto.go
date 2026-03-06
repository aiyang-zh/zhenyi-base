package zserialize

import (
	"google.golang.org/protobuf/proto"
)

func UnmarshalProto(body []byte, data proto.Message) error {
	err := proto.Unmarshal(body, data)
	return err
}

func MarshalProto(obj proto.Message) ([]byte, error) {
	body, err := proto.Marshal(obj)
	return body, err
}
