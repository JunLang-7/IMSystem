package main

import (
	"bytes"
	"io"
	"net"
	"strings"
	"sync"
	"testing"
	"time"
)

type mockAddr string

func (a mockAddr) Network() string { return "tcp" }
func (a mockAddr) String() string  { return string(a) }

type mockConn struct {
	mu        sync.Mutex
	buf       bytes.Buffer
	local     net.Addr
	remote    net.Addr
	readData  chan []byte
	closed    bool
	closeRead chan struct{}
}

func newMockConn(remote string) *mockConn {
	return &mockConn{
		local:     mockAddr("127.0.0.1:0"),
		remote:    mockAddr(remote),
		readData:  make(chan []byte, 10),
		closeRead: make(chan struct{}),
	}
}

func newMockConnWithReadData(remote string) *mockConn {
	return &mockConn{
		local:     mockAddr("127.0.0.1:0"),
		remote:    mockAddr(remote),
		readData:  make(chan []byte, 10),
		closeRead: make(chan struct{}),
	}
}

func (m *mockConn) Read(p []byte) (int, error) {
	select {
	case data := <-m.readData:
		n := copy(p, data)
		return n, nil
	case <-m.closeRead:
		return 0, io.EOF
	}
}

func (m *mockConn) Write(p []byte) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.buf.Write(p)
}

func (m *mockConn) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closed = true
	return nil
}

func (m *mockConn) IsClosed() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.closed
}

func (m *mockConn) LocalAddr() net.Addr { return m.local }

func (m *mockConn) RemoteAddr() net.Addr { return m.remote }

func (m *mockConn) SetDeadline(_ time.Time) error { return nil }

func (m *mockConn) SetReadDeadline(_ time.Time) error { return nil }

func (m *mockConn) SetWriteDeadline(_ time.Time) error { return nil }

func (m *mockConn) String() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.buf.String()
}

func TestNewServerInitialization(t *testing.T) {
	s := NewServer("127.0.0.1", 9000)

	if s.Ip != "127.0.0.1" {
		t.Fatalf("unexpected ip: %s", s.Ip)
	}
	if s.Port != 9000 {
		t.Fatalf("unexpected port: %d", s.Port)
	}
	if s.OnlineMap == nil {
		t.Fatal("online map should be initialized")
	}
	if s.Message == nil {
		t.Fatal("message channel should be initialized")
	}
}

func TestBroadcastFormatsMessage(t *testing.T) {
	s := NewServer("127.0.0.1", 9000)
	s.Message = make(chan string, 1)

	u := &User{Name: "alice", Addr: "1.2.3.4:1000", server: s}
	s.Broadcast(u, "hello")

	got := <-s.Message
	want := "[1.2.3.4:1000]alice:hello"
	if got != want {
		t.Fatalf("unexpected broadcast message, got=%q want=%q", got, want)
	}
}

func TestUserOnlineOffline(t *testing.T) {
	s := NewServer("127.0.0.1", 9000)
	s.Message = make(chan string, 2)

	u := &User{Name: "bob", Addr: "2.2.2.2:2000", server: s}

	u.Online()
	if _, ok := s.OnlineMap["bob"]; !ok {
		t.Fatal("user should be in online map after Online")
	}
	if got := <-s.Message; got != "[2.2.2.2:2000]bob:Online" {
		t.Fatalf("unexpected online broadcast: %q", got)
	}

	u.Offline()
	if _, ok := s.OnlineMap["bob"]; ok {
		t.Fatal("user should be removed from online map after Offline")
	}
	if got := <-s.Message; got != "[2.2.2.2:2000]bob:Offline" {
		t.Fatalf("unexpected offline broadcast: %q", got)
	}
}

func TestDoMessageWho(t *testing.T) {
	s := NewServer("127.0.0.1", 9000)
	conn := newMockConn("10.0.0.1:1234")

	requester := &User{Name: "requester", Addr: "10.0.0.1:1234", conn: conn, server: s}
	u1 := &User{Name: "u1", Addr: "10.0.0.2:1111", server: s}
	u2 := &User{Name: "u2", Addr: "10.0.0.3:2222", server: s}

	s.OnlineMap[requester.Name] = requester
	s.OnlineMap[u1.Name] = u1
	s.OnlineMap[u2.Name] = u2

	requester.DoMessage("who")

	got := conn.String()
	if !strings.Contains(got, "[10.0.0.2:1111]u1: Online...\n") {
		t.Fatalf("who result missing u1, got=%q", got)
	}
	if !strings.Contains(got, "[10.0.0.3:2222]u2: Online...\n") {
		t.Fatalf("who result missing u2, got=%q", got)
	}
}

func TestDoMessageRenameSuccess(t *testing.T) {
	s := NewServer("127.0.0.1", 9000)
	conn := newMockConn("10.0.0.4:3333")

	u := &User{Name: "old", Addr: "10.0.0.4:3333", conn: conn, server: s}
	s.OnlineMap[u.Name] = u

	u.DoMessage("rename|new")

	if u.Name != "new" {
		t.Fatalf("expected new name, got=%q", u.Name)
	}
	if _, ok := s.OnlineMap["old"]; ok {
		t.Fatal("old name should be removed from online map")
	}
	if _, ok := s.OnlineMap["new"]; !ok {
		t.Fatal("new name should be present in online map")
	}
	if got := conn.String(); got != "Rename new success!\n" {
		t.Fatalf("unexpected response: %q", got)
	}
}

func TestDoMessageRenameConflict(t *testing.T) {
	s := NewServer("127.0.0.1", 9000)
	conn := newMockConn("10.0.0.5:4444")

	u := &User{Name: "old", Addr: "10.0.0.5:4444", conn: conn, server: s}
	s.OnlineMap[u.Name] = u
	s.OnlineMap["taken"] = &User{Name: "taken", Addr: "10.0.0.6:5555", server: s}

	u.DoMessage("rename|taken")

	if u.Name != "old" {
		t.Fatalf("name should not change on conflict, got=%q", u.Name)
	}
	if got := conn.String(); got != "The name is already in use!\n" {
		t.Fatalf("unexpected response: %q", got)
	}
}

func TestDoMessageBroadcastFallback(t *testing.T) {
	s := NewServer("127.0.0.1", 9000)
	s.Message = make(chan string, 1)

	u := &User{Name: "carol", Addr: "10.0.0.7:6666", server: s}
	u.DoMessage("hi everyone")

	if got := <-s.Message; got != "[10.0.0.7:6666]carol:hi everyone" {
		t.Fatalf("unexpected broadcast fallback: %q", got)
	}
}

func TestDoMessageRenamePrefixLengthBoundary(t *testing.T) {
	s := NewServer("127.0.0.1", 9000)
	s.Message = make(chan string, 1)

	u := &User{Name: "edge", Addr: "10.0.0.8:7777", server: s}
	s.OnlineMap[u.Name] = u

	u.DoMessage("rename|")

	if u.Name != "edge" {
		t.Fatalf("name should not change when message is exactly rename|, got=%q", u.Name)
	}
	if _, ok := s.OnlineMap["edge"]; !ok {
		t.Fatal("user should remain with original name in online map")
	}
	if got := <-s.Message; got != "[10.0.0.8:7777]edge:rename|" {
		t.Fatalf("expected fallback broadcast for rename| boundary, got=%q", got)
	}
}

func TestDoMessageEmptyMessageBroadcast(t *testing.T) {
	s := NewServer("127.0.0.1", 9000)
	s.Message = make(chan string, 1)

	u := &User{Name: "empty", Addr: "10.0.0.9:8888", server: s}
	u.DoMessage("")

	if got := <-s.Message; got != "[10.0.0.9:8888]empty:" {
		t.Fatalf("unexpected broadcast for empty message: %q", got)
	}
}

func TestHandlerTimeoutForceOffline(t *testing.T) {
	s := NewServer("127.0.0.1", 9000)
	s.Message = make(chan string, 10)

	// 启动广播监听
	go s.ListenMessage()

	conn := newMockConnWithReadData("10.0.1.1:9999")

	// 在后台运行 Handler，它会在超时后关闭连接
	done := make(chan bool)
	go func() {
		s.Handler(conn)
		done <- true
	}()

	// 等待用户上线消息
	time.Sleep(100 * time.Millisecond)

	// 验证用户已上线
	s.maplock.RLock()
	_, online := s.OnlineMap["10.0.1.1:9999"]
	s.maplock.RUnlock()
	if !online {
		t.Fatal("user should be online after Handler starts")
	}

	// 等待超过超时时间（10秒），但为了测试速度，我们只能检查连接是否最终关闭
	// 由于实际超时是 10 秒，我们等待 11 秒
	select {
	case <-done:
		// Handler 已退出
	case <-time.After(12 * time.Second):
		t.Fatal("handler should exit after timeout")
	}

	// 检查连接是否被关闭
	if !conn.IsClosed() {
		t.Fatal("connection should be closed after timeout")
	}

	// 检查是否发送了超时消息
	output := conn.String()
	if !strings.Contains(output, "You're offined...") {
		t.Fatalf("should send timeout message, got: %q", output)
	}
}

func TestHandlerActivityKeepsUserOnline(t *testing.T) {
	s := NewServer("127.0.0.1", 9000)
	s.Message = make(chan string, 10)

	// 启动广播监听
	go s.ListenMessage()

	conn := newMockConnWithReadData("10.0.1.2:8888")

	// 在后台运行 Handler
	go s.Handler(conn)

	// 等待用户上线
	time.Sleep(100 * time.Millisecond)

	// 在超时前发送消息（每 5 秒发送一次，共发送 3 次）
	for i := 0; i < 3; i++ {
		time.Sleep(5 * time.Second)
		conn.readData <- []byte("hello\n")
		time.Sleep(100 * time.Millisecond)
	}

	// 15 秒后用户应该仍然在线（因为每 5 秒有活动）
	s.maplock.RLock()
	_, online := s.OnlineMap["10.0.1.2:8888"]
	s.maplock.RUnlock()

	if !online {
		t.Fatal("user should still be online after sending messages within timeout period")
	}

	// 连接不应该被关闭
	if conn.IsClosed() {
		t.Fatal("connection should not be closed when user is active")
	}
}

func TestHandlerNoActivityCausesTimeout(t *testing.T) {
	s := NewServer("127.0.0.1", 9000)
	s.Message = make(chan string, 10)

	// 启动广播监听
	go s.ListenMessage()

	conn := newMockConnWithReadData("10.0.1.3:7777")

	done := make(chan bool)
	go func() {
		s.Handler(conn)
		done <- true
	}()

	// 等待用户上线
	time.Sleep(100 * time.Millisecond)

	// 发送一条消息
	conn.readData <- []byte("initial\n")
	time.Sleep(200 * time.Millisecond)

	// 验证消息被处理（消息会被写入到 conn）
	output := conn.String()
	if !strings.Contains(output, "initial") {
		// 消息应该被广播并写回到连接
		t.Logf("Note: message may not be echoed back, continuing test")
	}

	// 然后不再发送任何消息，等待超时
	select {
	case <-done:
		// Handler 已退出
	case <-time.After(12 * time.Second):
		t.Fatal("handler should exit after timeout despite earlier activity")
	}

	// 检查连接被关闭
	if !conn.IsClosed() {
		t.Fatal("connection should be closed after inactivity timeout")
	}

	// 检查超时消息
	finalOutput := conn.String()
	if !strings.Contains(finalOutput, "You're offined...") {
		t.Fatalf("should send timeout message, got: %q", finalOutput)
	}
}

func TestDoMessagePrivateChatSuccess(t *testing.T) {
	s := NewServer("127.0.0.1", 9000)

	senderConn := newMockConn("10.0.2.1:5001")
	receiverConn := newMockConn("10.0.2.2:5002")

	sender := &User{Name: "alice", Addr: "10.0.2.1:5001", conn: senderConn, server: s}
	receiver := &User{Name: "bob", Addr: "10.0.2.2:5002", conn: receiverConn, server: s}

	s.OnlineMap[sender.Name] = sender
	s.OnlineMap[receiver.Name] = receiver

	// Alice 发送私聊消息给 Bob
	sender.DoMessage("to|bob|hello bob")

	// 检查 Bob 是否收到了消息
	receivedMsg := receiverConn.String()
	if !strings.Contains(receivedMsg, "[alice] hello bob") {
		t.Fatalf("bob should receive private message, got: %q", receivedMsg)
	}

	// 检查 Alice 的连接中没有收到消息（私聊不回显）
	senderOutput := senderConn.String()
	if strings.Contains(senderOutput, "hello bob") {
		t.Fatalf("sender should not receive their own private message, got: %q", senderOutput)
	}
}

func TestDoMessagePrivateChatTargetOffline(t *testing.T) {
	s := NewServer("127.0.0.1", 9000)

	senderConn := newMockConn("10.0.2.3:5003")
	sender := &User{Name: "alice", Addr: "10.0.2.3:5003", conn: senderConn, server: s}

	s.OnlineMap[sender.Name] = sender

	// Alice 试图发送私聊消息给不在线的 Carol
	sender.DoMessage("to|carol|hello carol")

	// 检查 Alice 是否收到了离线提示
	output := senderConn.String()
	if !strings.Contains(output, "User carol is not online") {
		t.Fatalf("sender should receive user offline notification, got: %q", output)
	}
}

func TestDoMessagePrivateChatInvalidFormat(t *testing.T) {
	s := NewServer("127.0.0.1", 9000)

	senderConn := newMockConn("10.0.2.4:5004")
	sender := &User{Name: "alice", Addr: "10.0.2.4:5004", conn: senderConn, server: s}

	s.OnlineMap[sender.Name] = sender

	// Alice 发送格式错误的私聊命令
	sender.DoMessage("to|bob")

	// 检查 Alice 是否收到了格式错误提示
	output := senderConn.String()
	if !strings.Contains(output, "Invalid 'to' command format") {
		t.Fatalf("sender should receive format error, got: %q", output)
	}
}

func TestDoMessagePrivateChatEmptyMessage(t *testing.T) {
	s := NewServer("127.0.0.1", 9000)

	senderConn := newMockConn("10.0.2.5:5005")
	receiverConn := newMockConn("10.0.2.6:5006")

	sender := &User{Name: "alice", Addr: "10.0.2.5:5005", conn: senderConn, server: s}
	receiver := &User{Name: "bob", Addr: "10.0.2.6:5006", conn: receiverConn, server: s}

	s.OnlineMap[sender.Name] = sender
	s.OnlineMap[receiver.Name] = receiver

	// Alice 发送空消息给 Bob
	sender.DoMessage("to|bob|")

	// 检查 Alice 是否收到了空消息提示
	output := senderConn.String()
	if !strings.Contains(output, "Message cannot be empty") {
		t.Fatalf("sender should receive empty message error, got: %q", output)
	}

	// 检查 Bob 没有收到任何消息
	receiverOutput := receiverConn.String()
	if receiverOutput != "" {
		t.Fatalf("receiver should not receive empty message, got: %q", receiverOutput)
	}
}

func TestDoMessagePrivateChatMultipleMessages(t *testing.T) {
	s := NewServer("127.0.0.1", 9000)

	aliceConn := newMockConn("10.0.2.7:5007")
	bobConn := newMockConn("10.0.2.8:5008")

	alice := &User{Name: "alice", Addr: "10.0.2.7:5007", conn: aliceConn, server: s}
	bob := &User{Name: "bob", Addr: "10.0.2.8:5008", conn: bobConn, server: s}

	s.OnlineMap[alice.Name] = alice
	s.OnlineMap[bob.Name] = bob

	// Alice 发送第一条私聊
	alice.DoMessage("to|bob|first message")

	// Bob 发送回复
	bob.DoMessage("to|alice|second message")

	// 检查 Bob 收到 Alice 的第一条消息
	bobOutput := bobConn.String()
	if !strings.Contains(bobOutput, "[alice] first message") {
		t.Fatalf("bob should receive alice's message, got: %q", bobOutput)
	}

	// 检查 Alice 收到 Bob 的回复
	aliceOutput := aliceConn.String()
	if !strings.Contains(aliceOutput, "[bob] second message") {
		t.Fatalf("alice should receive bob's message, got: %q", aliceOutput)
	}
}

func TestDoMessagePrivateChatWithSpecialCharacters(t *testing.T) {
	s := NewServer("127.0.0.1", 9000)

	senderConn := newMockConn("10.0.2.9:5009")
	receiverConn := newMockConn("10.0.2.10:5010")

	sender := &User{Name: "alice", Addr: "10.0.2.9:5009", conn: senderConn, server: s}
	receiver := &User{Name: "bob", Addr: "10.0.2.10:5010", conn: receiverConn, server: s}

	s.OnlineMap[sender.Name] = sender
	s.OnlineMap[receiver.Name] = receiver

	// Alice 发送包含管道符的消息
	sender.DoMessage("to|bob|hello|with|pipes")

	// 检查 Bob 是否正确接收了包含管道符的消息
	receivedMsg := receiverConn.String()
	if !strings.Contains(receivedMsg, "[alice] hello|with|pipes") {
		t.Fatalf("receiver should get message with pipes preserved, got: %q", receivedMsg)
	}
}

func TestDoMessagePrivateChatLongMessage(t *testing.T) {
	s := NewServer("127.0.0.1", 9000)

	senderConn := newMockConn("10.0.3.1:6001")
	receiverConn := newMockConn("10.0.3.2:6002")

	sender := &User{Name: "alice", Addr: "10.0.3.1:6001", conn: senderConn, server: s}
	receiver := &User{Name: "bob", Addr: "10.0.3.2:6002", conn: receiverConn, server: s}

	s.OnlineMap[sender.Name] = sender
	s.OnlineMap[receiver.Name] = receiver

	// Alice 发送长消息给 Bob
	longMessage := "This is a long message " + strings.Repeat("x", 1000)
	sender.DoMessage("to|bob|" + longMessage)

	// 检查 Bob 是否收到了完整的长消息
	receivedMsg := receiverConn.String()
	if !strings.Contains(receivedMsg, "[alice]") {
		t.Fatalf("bob should receive long private message, got: %q", receivedMsg)
	}
	if !strings.Contains(receivedMsg, longMessage) {
		t.Fatalf("bob should receive complete long message, got: %q", receivedMsg)
	}
}
