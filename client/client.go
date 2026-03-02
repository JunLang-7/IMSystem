package main

import (
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
