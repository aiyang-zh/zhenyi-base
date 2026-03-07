package main

import (
	"github.com/aiyang-zh/zhenyi-base/zserver"
)

func main() {
	s := zserver.New(zserver.WithAddr(":9001"))
	s.Handle(1, func(req *zserver.Request) {
		req.Reply(1, req.Data())
	})
	s.Run()
}
