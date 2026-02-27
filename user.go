package main

import "net"

type User struct {
	Name string
	Addr string
	C    chan string
	conn net.Conn
}

// NewUser 创建一个新的用户实例
func NewUser(conn net.Conn) *User {
	useraddr := conn.RemoteAddr().String()
	user := &User{
		Name: useraddr,
		Addr: useraddr,
		C:    make(chan string),
		conn: conn,
	}
	// 启动监听当前User channel消息的goroutine
	go user.ListenMessage()

	return user
}

// ListenMessage 监听用户消息 并发送给客户端
func (u *User) ListenMessage() {
	for {
		msg := <-u.C
		u.conn.Write([]byte(msg + "\n"))
	}
}
