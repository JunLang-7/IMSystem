package main

import (
	"flag"
	"fmt"
)

var serverIp string
var serverPort int

func init() {
	// ./client -ip 127.0.0.1 -port 8000
	flag.StringVar(&serverIp, "ip", "127.0.0.1", "server ip")
	flag.IntVar(&serverPort, "port", 8000, "server port")
}

func main() {
	// 解析命令行参数
	flag.Parse()

	client := NewClient(serverIp, serverPort)
	if client == nil {
		fmt.Println("Failed to connect to server.")
		return
	}
	// 启动一个goroutine去处理server的回执消息
	go client.DealResponse()

	fmt.Println(">>>Connect to server...")

	// 启动客户端的业务
	client.Run()
}
