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
	flag 	   int // 0 for public chat, 1 for private chat, 2 for update username
}

func NewClient(serverIp string, serverPort int) *Client {
	client := &Client{
		ServerIp:   serverIp,
		ServerPort: serverPort,
		flag: 999,
	}
	conn, err := net.Dial("tcp", net.JoinHostPort(serverIp, fmt.Sprintf("%d", serverPort)))
	if err != nil {
		fmt.Println("net.Dial error", err)
		return nil
	}
	client.conn = conn
	return client
}

// menu 显示菜单
func (c *Client) menu() bool {
	fmt.Println("1. Public chat")
	fmt.Println("2. Private chat")
	fmt.Println("3. Update username")
	fmt.Println("0. Exit")

	var choice int
	fmt.Scanln(&choice)
	if choice >= 0 && choice <= 3 {
		c.flag = choice
		return true
	} else {
		fmt.Println("Invalid choice, please try again.")
		return false
	}
}

// Run 启动客户端
func (c *Client) Run() {
	for c.flag != 0 {
		for c.menu() != true {

		}
		// 根据用户的选择，执行相应的操作
		switch c.flag {
		case 1:
			// public chat
			fmt.Println(">>>Public chat mode, please enter your message:")
		case 2:
			// private chat
			fmt.Println(">>>Private chat mode, please enter the username of the person you want to chat with:")
		case 3:
			// update username
			fmt.Println(">>>Update username mode, please enter your new username:")
		}
	}
}
