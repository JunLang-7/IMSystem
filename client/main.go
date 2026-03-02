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
	fmt.Println(">>>Connect to server...")
	client.Run()
}
