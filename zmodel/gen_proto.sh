
protoc -I=. --go_out=../../ --go-vtproto_out=../../ --go-vtproto_opt=features=all ./msg.proto

gofmt -w .