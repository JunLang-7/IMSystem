package main

import (
	"flag"
	"fmt"
	"net"
)

type Client struct {
	ServerIp   string
	ServerPort int
	Name       string
	conn       net.Conn
}

func NewClient(serverIp string, serverPort int) *Client {
	client := &Client{
		ServerIp:   serverIp,
		ServerPort: serverPort,
	}
	conn, err := net.Dial("tcp", net.JoinHostPort(serverIp, fmt.Sprintf("%d", serverPort)))
	if err != nil {
		fmt.Println("net.Dial error", err)
		return nil
	}
	client.conn = conn
	return client
}

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
	for {
	}
}
