
..\tools\proto\win\protoc.exe -I=. --plugin=protoc-gen-go=..\tools\proto\win\protoc-gen-go.exe --go_out=../../ --go-vtproto_out=../../ --go-vtproto_opt=features=all ./msg.proto

gofmt -w .