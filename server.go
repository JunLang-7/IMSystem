package main

import (
	"fmt"
	"net"
)

type Sever struct {
	Ip   string
	Port int
}

// NewServer 创建一个新的服务器实例
func NewServer(ip string, port int) *Sever {
	return &Sever{
		Ip:   ip,
		Port: port,
	}
}

// Start 启动服务器
func (s *Sever) Start() {
	// socket listen
	listener, err := net.Listen("tcp", fmt.Sprintf("%s:%d", s.Ip, s.Port))
	if err != nil {
		fmt.Println("Failed to start server:", err)
		return
	}
	// close
	defer listener.Close()

	for {
		// accept
		conn, err := listener.Accept()
		if err != nil {
			fmt.Println("Failed to accept connection:", err)
			continue
		}
		// do handler
		go s.Handler(conn)
	}
}

// Handler 处理连接
func (s *Sever) Handler(conn net.Conn) {
	fmt.Println("Connection established from", conn.RemoteAddr().String())
}
