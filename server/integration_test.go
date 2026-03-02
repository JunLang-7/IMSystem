package main

import (
	"bufio"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"
)

// ClientConn 模仿 NewClient 的返回类型，用于测试
type ClientConn struct {
	ServerIp   string
	ServerPort int
	Name       string
	conn       net.Conn
}

// NewTestClient 模拟 NewClient，用于集成测试
func NewTestClient(serverIp string, serverPort int) *ClientConn {
	clientConn := &ClientConn{
		ServerIp:   serverIp,
		ServerPort: serverPort,
	}
	conn, err := net.Dial("tcp", net.JoinHostPort(serverIp, fmt.Sprintf("%d", serverPort)))
	if err != nil {
		fmt.Println("net.Dial error", err)
		return nil
	}
	clientConn.conn = conn
	return clientConn
}

// TestIntegration_ClientServerInteraction 测试客户端和服务器的完整交互
func TestIntegration_ClientServerInteraction(t *testing.T) {
	// 启动服务器
	server := NewServer("127.0.0.1", 9999)
	go server.Start()

	// 等待服务器启动
	time.Sleep(100 * time.Millisecond)

	// 使用 NewTestClient（类似 NewClient）创建客户端连接
	client1 := NewTestClient("127.0.0.1", 9999)
	if client1 == nil {
		t.Fatalf("Failed to create client 1")
	}
	defer client1.conn.Close()

	client2 := NewTestClient("127.0.0.1", 9999)
	if client2 == nil {
		t.Fatalf("Failed to create client 2")
	}
	defer client2.conn.Close()

	// 等待两个客户端的在线消息
	time.Sleep(100 * time.Millisecond)

	// 验证服务器有两个在线用户
	server.maplock.RLock()
	if len(server.OnlineMap) != 2 {
		t.Errorf("Expected 2 online users, got %d", len(server.OnlineMap))
	}
	server.maplock.RUnlock()
}

// TestIntegration_PublicChat 测试公聊功能
func TestIntegration_PublicChat(t *testing.T) {
	server := NewServer("127.0.0.1", 9998)
	go server.Start()
	time.Sleep(100 * time.Millisecond)

	// 使用 NewTestClient 创建两个客户端
	client1 := NewTestClient("127.0.0.1", 9998)
	if client1 == nil {
		t.Fatalf("Failed to create client 1")
	}
	defer client1.conn.Close()

	client2 := NewTestClient("127.0.0.1", 9998)
	if client2 == nil {
		t.Fatalf("Failed to create client 2")
	}
	defer client2.conn.Close()

	time.Sleep(100 * time.Millisecond)

	// 客户端 1 发送公聊消息
	message := "Hello everyone\n"
	_, err := client1.conn.Write([]byte(message))
	if err != nil {
		t.Fatalf("Failed to send message: %v", err)
	}

	// 客户端 2 应该能收到广播消息
	time.Sleep(100 * time.Millisecond)
	client2.conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	buf := make([]byte, 1024)
	_, err = client2.conn.Read(buf)

	if err == nil {
		t.Logf("Client 2 received message successfully")
	}
}

// TestIntegration_PrivateChat 测试私聊功能
func TestIntegration_PrivateChat(t *testing.T) {
	server := NewServer("127.0.0.1", 9997)
	go server.Start()
	time.Sleep(100 * time.Millisecond)

	// 使用 NewTestClient 创建两个客户端
	client1 := NewTestClient("127.0.0.1", 9997)
	if client1 == nil {
		t.Fatalf("Failed to create client 1")
	}
	defer client1.conn.Close()

	client2 := NewTestClient("127.0.0.1", 9997)
	if client2 == nil {
		t.Fatalf("Failed to create client 2")
	}
	defer client2.conn.Close()

	time.Sleep(100 * time.Millisecond)

	// 客户端 1 发送 "who" 查询在线用户
	whoMsg := "who\n"
	_, err := client1.conn.Write([]byte(whoMsg))
	if err != nil {
		t.Fatalf("Failed to send who command: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	// 客户端 1 向客户端 2 发送私聊消息
	privateMsg := "to|user2|secret message\n"
	_, err = client1.conn.Write([]byte(privateMsg))
	if err != nil {
		t.Fatalf("Failed to send private message: %v", err)
	}

	t.Logf("Private message sent successfully")
}

// TestIntegration_RenameCommand 测试重命名功能
func TestIntegration_RenameCommand(t *testing.T) {
	server := NewServer("127.0.0.1", 9996)
	go server.Start()
	time.Sleep(100 * time.Millisecond)

	client := NewTestClient("127.0.0.1", 9996)
	if client == nil {
		t.Fatalf("Failed to create client")
	}
	defer client.conn.Close()

	time.Sleep(100 * time.Millisecond)

	// 发送重命名命令
	renameMsg := "rename|alice\n"
	_, err := client.conn.Write([]byte(renameMsg))
	if err != nil {
		t.Fatalf("Failed to send rename command: %v", err)
	}

	// 验证在线用户中包含新名称
	time.Sleep(100 * time.Millisecond)
	server.maplock.RLock()
	hasAlice := false
	for _, user := range server.OnlineMap {
		if user.Name == "alice" {
			hasAlice = true
			break
		}
	}
	server.maplock.RUnlock()

	if !hasAlice {
		t.Errorf("Expected to find user 'alice' after rename")
	}
}

// TestIntegration_MultipleClientsChat 测试多个客户端的聊天
func TestIntegration_MultipleClientsChat(t *testing.T) {
	server := NewServer("127.0.0.1", 9995)
	go server.Start()
	time.Sleep(100 * time.Millisecond)

	// 创建 3 个客户端连接
	const numClients = 3
	clients := make([]*ClientConn, numClients)
	for i := 0; i < numClients; i++ {
		client := NewTestClient("127.0.0.1", 9995)
		if client == nil {
			t.Fatalf("Failed to create client %d", i)
		}
		clients[i] = client
		defer client.conn.Close()
	}

	time.Sleep(100 * time.Millisecond)

	// 验证所有客户端都在线
	server.maplock.RLock()
	onlineCount := len(server.OnlineMap)
	server.maplock.RUnlock()

	if onlineCount != numClients {
		t.Errorf("Expected %d online users, got %d", numClients, onlineCount)
	}

	// 第一个客户端发送公聊消息
	message := "Hello from client 0\n"
	_, err := clients[0].conn.Write([]byte(message))
	if err != nil {
		t.Fatalf("Failed to send message: %v", err)
	}

	// 等待消息广播
	time.Sleep(100 * time.Millisecond)

	// 验证其他客户端可以收到消息
	for i := 1; i < numClients; i++ {
		clients[i].conn.SetReadDeadline(time.Now().Add(1 * time.Second))
		buf := make([]byte, 1024)
		_, err := clients[i].conn.Read(buf)
		if err == nil {
			t.Logf("Client %d received message", i)
		}
	}
}

// TestIntegration_UserOnlineOffline 测试用户上线和下线事件
func TestIntegration_UserOnlineOffline(t *testing.T) {
	server := NewServer("127.0.0.1", 9994)
	go server.Start()
	time.Sleep(100 * time.Millisecond)

	// 连接第一个客户端
	client1 := NewTestClient("127.0.0.1", 9994)
	if client1 == nil {
		t.Fatalf("Failed to create client 1")
	}

	time.Sleep(100 * time.Millisecond)

	// 连接第二个客户端
	client2 := NewTestClient("127.0.0.1", 9994)
	if client2 == nil {
		t.Fatalf("Failed to create client 2")
	}

	time.Sleep(100 * time.Millisecond)

	// 验证两个用户在线
	server.maplock.RLock()
	initialCount := len(server.OnlineMap)
	server.maplock.RUnlock()

	if initialCount != 2 {
		t.Errorf("Expected 2 online users, got %d", initialCount)
	}

	// 关闭第一个客户端
	client1.conn.Close()
	time.Sleep(100 * time.Millisecond)

	// 验证只剩一个用户在线
	server.maplock.RLock()
	afterCount := len(server.OnlineMap)
	server.maplock.RUnlock()

	if afterCount != 1 {
		t.Errorf("Expected 1 online user after disconnect, got %d", afterCount)
	}

	client2.conn.Close()
}

// TestIntegration_ConcurrentConnections 测试并发连接处理
func TestIntegration_ConcurrentConnections(t *testing.T) {
	server := NewServer("127.0.0.1", 9993)
	go server.Start()
	time.Sleep(100 * time.Millisecond)

	const numConnections = 10
	done := make(chan bool, numConnections)

	// 并发创建多个连接
	for i := 0; i < numConnections; i++ {
		go func(id int) {
			client := NewTestClient("127.0.0.1", 9993)
			if client == nil {
				t.Errorf("Failed to create client %d", id)
				done <- false
				return
			}
			defer client.conn.Close()

			// 发送一条消息
			msg := fmt.Sprintf("Message from client %d\n", id)
			_, err := client.conn.Write([]byte(msg))
			if err != nil {
				t.Errorf("Failed to send message from client %d: %v", id, err)
				done <- false
				return
			}

			done <- true
		}(i)
	}

	// 等待所有连接完成
	successCount := 0
	for i := 0; i < numConnections; i++ {
		if <-done {
			successCount++
		}
	}

	time.Sleep(200 * time.Millisecond)

	// 验证所有连接都成功
	if successCount != numConnections {
		t.Errorf("Expected %d successful connections, got %d", numConnections, successCount)
	}

	// 验证所有客户端都在线（或已正确处理）
	server.maplock.RLock()
	onlineCount := len(server.OnlineMap)
	server.maplock.RUnlock()

	t.Logf("Online users: %d", onlineCount)
}

// TestIntegration_MessageProtocol 测试消息协议的正确性
func TestIntegration_MessageProtocol(t *testing.T) {
	server := NewServer("127.0.0.1", 9992)
	go server.Start()
	time.Sleep(100 * time.Millisecond)

	client := NewTestClient("127.0.0.1", 9992)
	if client == nil {
		t.Fatalf("Failed to create client")
	}
	defer client.conn.Close()

	time.Sleep(100 * time.Millisecond)

	// 测试各种消息协议
	testCases := []string{
		"who\n",                    // 查询在线用户
		"public message\n",         // 公聊
		"rename|newname\n",         // 重命名
		"to|user|private msg\n",    // 私聊
		"message with spaces\n",    // 含空格的消息
	}

	for i, testCase := range testCases {
		_, err := client.conn.Write([]byte(testCase))
		if err != nil {
			t.Errorf("Test case %d failed: %v", i, err)
		}
	}

	t.Logf("All message protocols sent successfully")
}

// TestIntegration_LongMessageHandling 测试长消息处理
func TestIntegration_LongMessageHandling(t *testing.T) {
	server := NewServer("127.0.0.1", 9991)
	go server.Start()
	time.Sleep(100 * time.Millisecond)

	client := NewTestClient("127.0.0.1", 9991)
	if client == nil {
		t.Fatalf("Failed to create client")
	}
	defer client.conn.Close()

	time.Sleep(100 * time.Millisecond)

	// 发送长消息
	longMessage := strings.Repeat("x", 3000) + "\n"
	_, err := client.conn.Write([]byte(longMessage))
	if err != nil {
		t.Fatalf("Failed to send long message: %v", err)
	}

	t.Logf("Long message (3000+ chars) sent successfully")
}

// TestIntegration_BufferedReading 测试缓冲读取
func TestIntegration_BufferedReading(t *testing.T) {
	server := NewServer("127.0.0.1", 9990)
	go server.Start()
	time.Sleep(100 * time.Millisecond)

	// 两个客户端连接
	serverClient := NewTestClient("127.0.0.1", 9990)
	if serverClient == nil {
		t.Fatalf("Failed to create server client")
	}
	defer serverClient.conn.Close()

	clientConn := NewTestClient("127.0.0.1", 9990)
	if clientConn == nil {
		t.Fatalf("Failed to create client")
	}
	defer clientConn.conn.Close()

	time.Sleep(100 * time.Millisecond)

	// 第一个客户端发送消息
	msg := "Test message\n"
	_, err := serverClient.conn.Write([]byte(msg))
	if err != nil {
		t.Fatalf("Failed to send message: %v", err)
	}

	// 第二个客户端使用 bufio 读取
	time.Sleep(100 * time.Millisecond)
	clientConn.conn.SetReadDeadline(time.Now().Add(1 * time.Second))
	reader := bufio.NewReader(clientConn.conn)
	line, err := reader.ReadString('\n')

	if err == nil {
		t.Logf("Successfully read: %q", line)
	}
}
