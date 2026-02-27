package main

import (
	"fmt"
	"io"
	"net"
	"sync"
)

type Sever struct {
	Ip   string
	Port int
	// 在线用户列表
	OnlineMap map[string]*User
	maplock   sync.RWMutex
	// 消息广播的channel
	Message chan string
}

// NewServer 创建一个新的服务器实例
func NewServer(ip string, port int) *Sever {
	return &Sever{
		Ip:        ip,
		Port:      port,
		OnlineMap: make(map[string]*User),
		Message:   make(chan string),
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

	// 启动监听Message的goroutine
	go s.ListenMessage()

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

func (s *Sever) ListenMessage() {
	for {
		msg := <-s.Message
		// 将消息发送给在线的用户
		s.maplock.Lock()
		for _, user := range s.OnlineMap {
			user.C <- msg
		}
		s.maplock.Unlock()
	}
}

// Handler 处理连接
func (s *Sever) Handler(conn net.Conn) {
	// 创建用户
	user := NewUser(conn)

	// 用户上线 加入在线列表
	s.maplock.Lock()
	s.OnlineMap[user.Name] = user
	s.maplock.Unlock()

	// 广播用户上线消息
	s.Broadcast(user, "Online")

	// 接收客户端发送的消息
	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := conn.Read(buf)
			if n == 0 {
				s.Broadcast(user, "Offline")
				return
			}
			if err != nil && err != io.EOF {
				fmt.Println("Conn Read Err: ", err)
				return
			}

			// 提取用户的消息（去除换行符）
			msg := string(buf[:n-1])
			s.Broadcast(user, msg)
		}
	}()

	// 阻塞当前handler
	select {}
}

// Broadcast 广播消息
func (s *Sever) Broadcast(user *User, msg string) {
	sendmsg := "[" + user.Addr + "]" + user.Name + ":" + msg
	s.Message <- sendmsg
}
