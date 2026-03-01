package main

import (
	"net"
	"strings"
)

type User struct {
	Name   string
	Addr   string
	C      chan string
	conn   net.Conn
	server *Server
}

// NewUser 创建一个新的用户实例
func NewUser(conn net.Conn, server *Server) *User {
	useraddr := conn.RemoteAddr().String()
	user := &User{
		Name:   useraddr,
		Addr:   useraddr,
		C:      make(chan string),
		conn:   conn,
		server: server,
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

// Online 用户上线
func (u *User) Online() {
	// 用户上线 加入在线列表
	u.server.maplock.Lock()
	u.server.OnlineMap[u.Name] = u
	u.server.maplock.Unlock()

	// 广播用户上线消息
	u.server.Broadcast(u, "Online")
}

// Offline 用户下线
func (u *User) Offline() {
	// 用户下线 从在线列表中移除
	u.server.maplock.Lock()
	delete(u.server.OnlineMap, u.Name)
	u.server.maplock.Unlock()

	// 广播用户下线消息
	u.server.Broadcast(u, "Offline")
}

// DoMessage 用户处理消息
func (u *User) DoMessage(msg string) {
	if msg == "who" {
		// 查询当前在线用户都有哪些
		u.server.maplock.Lock()
		for _, user := range u.server.OnlineMap {
			onlineMsg := "[" + user.Addr + "]" + user.Name + ": " + "Online...\n"
			u.SendMessage(onlineMsg)
		}
		u.server.maplock.Unlock()
	} else if len(msg) > 7 && msg[:7] == "rename|" {
		// rename|newName
		newName := strings.Split(msg, "|")[1]
		_, ok := u.server.OnlineMap[newName]
		if ok {
			u.SendMessage("The name is already in use!\n")
		} else {
			u.server.maplock.Lock()
			delete(u.server.OnlineMap, u.Name)
			u.server.OnlineMap[newName] = u
			u.server.maplock.Unlock()
			u.Name = newName
			u.SendMessage("Rename " + u.Name + " success!\n")
		}
	} else if len(msg) > 4 && msg[:3] == "to|" {
		// to|targetName|message
		// 获取对方用户名
		parts := strings.Split(msg, "|")
		if len(parts) < 3 {
			u.SendMessage("Invalid 'to' command format. Usage: to|targetName|message\n")
			return
		}
		targetName := parts[1]

		// 获取目标用户实例
		u.server.maplock.Lock()
		targetUser, exists := u.server.OnlineMap[targetName]
		u.server.maplock.Unlock()

		if !exists {
			u.SendMessage("User " + targetName + " is not online.\n")
			return
		}

		// 发送消息给目标用户
		message := strings.Join(parts[2:], "|")
		if message == "" {
			u.SendMessage("Message cannot be empty.\n")
			return
		}
		targetUser.SendMessage("[" + u.Name + "] " + message + "\n")
	} else {
		// 广播用户消息
		u.server.Broadcast(u, msg)
	}
}

// SendMessage 发送消息给当前用户
func (u *User) SendMessage(msg string) {
	u.conn.Write([]byte(msg))
}
