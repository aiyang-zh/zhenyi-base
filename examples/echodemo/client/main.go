package main

import (
	"bufio"
	"bytes"
	"fmt"
	"os"

	"github.com/aiyang-zh/zhenyi-base/ziface"
	"github.com/aiyang-zh/zhenyi-base/znet"
	"github.com/aiyang-zh/zhenyi-base/ztcp"
)

func main() {
	client, err := ztcp.NewClient("127.0.0.1:9001")
	if err != nil {
		fmt.Printf("connect failed: %v\n", err)
		return
	}
	defer client.Close()

	client.SetReadCall(func(msg ziface.IWireMessage) {
		fmt.Printf("echo: %s\n", string(msg.GetMessageData()))
	})
	go client.Read()

	scanner := bufio.NewScanner(os.Stdin)
	fmt.Println("输入任意内容，服务端会原样回显（Ctrl+D 退出）")
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		client.SendMsg(&znet.NetMessage{MsgId: 1, Data: bytes.Clone(line)})
	}
}
