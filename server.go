package main

import (
	"fmt"
	"io"
	"net"
	"sync"
	"time"
)

type Server struct {
	Ip        string
	Port      int
	OnlineMap map[string]*User // 在线用户列表
	maplock   sync.RWMutex
	Message   chan string // 消息广播的channel
}

// NewServer 创建一个新的服务器实例
func NewServer(ip string, port int) *Server {
	return &Server{
		Ip:        ip,
		Port:      port,
		OnlineMap: make(map[string]*User),
		Message:   make(chan string),
	}
}

// Start 启动服务器
func (s *Server) Start() {
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

func (s *Server) ListenMessage() {
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
func (s *Server) Handler(conn net.Conn) {
	// 创建用户
	user := NewUser(conn, s)

	// 用户上线
	user.Online()

	// 监听用户是否活跃
	isLive := make(chan bool)

	// 接收客户端发送的消息
	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := conn.Read(buf)
			if n == 0 {
				user.Offline()
				return
			}
			if err != nil && err != io.EOF {
				fmt.Println("Conn Read Err: ", err)
				return
			}

			// 提取用户的消息（去除换行符）
			msg := string(buf[:n-1])

			user.DoMessage(msg)

			// 用户的任意消息都代表当前用户是活跃的
			isLive <- true
		}
	}()

	// 阻塞当前handler
	for {
		select {
		case <-isLive:
			// 当前用户是活跃的，继续阻塞
			// Do nothing, just wait for the next message or timeout
			// 这里不需要任何操作，因为我们只是想重置定时器
		case <-time.After(time.Second * 10):
			// 超时强制关闭连接
			user.SendMessage("You're offined...\n")
			conn.Close()
			return
		}
	}
}

// Broadcast 广播消息
func (s *Server) Broadcast(user *User, msg string) {
	sendmsg := "[" + user.Addr + "]" + user.Name + ":" + msg
	s.Message <- sendmsg
}
