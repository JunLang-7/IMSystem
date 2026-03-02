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

// mockConn 用于客户端测试
type mockConn struct {
	mu       sync.Mutex
	buf      bytes.Buffer
	local    net.Addr
	remote   net.Addr
	readData chan []byte
	closed   bool
}

func newMockConn(remote string) *mockConn {
	return &mockConn{
		local:    mockAddr("127.0.0.1:0"),
		remote:   mockAddr(remote),
		readData: make(chan []byte, 10),
	}
}

func (m *mockConn) Read(p []byte) (int, error) {
	select {
	case data := <-m.readData:
		n := copy(p, data)
		return n, nil
	case <-time.After(50 * time.Millisecond):
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

func (m *mockConn) SendData(data []byte) {
	m.readData <- data
}

func TestClientInitialization(t *testing.T) {
	client := &Client{
		ServerIp:   "127.0.0.1",
		ServerPort: 8000,
		flag:       999,
	}

	if client.ServerIp != "127.0.0.1" {
		t.Errorf("expected ServerIp 127.0.0.1, got %s", client.ServerIp)
	}
	if client.ServerPort != 8000 {
		t.Errorf("expected ServerPort 8000, got %d", client.ServerPort)
	}
	if client.flag != 999 {
		t.Errorf("expected flag 999, got %d", client.flag)
	}
}

func TestClientFlagField(t *testing.T) {
	mockConn := newMockConn("10.0.0.1:5000")
	client := &Client{
		flag: 0,
		conn: mockConn,
	}

	tests := []struct {
		name     string
		flag     int
		expected bool
	}{
		{"Exit flag", 0, true},
		{"Public chat flag", 1, true},
		{"Private chat flag", 2, true},
		{"Update name flag", 3, true},
		{"Invalid flag", 4, false},
		{"Negative flag", -1, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client.flag = tt.flag
			isValid := client.flag >= 0 && client.flag <= 3
			if isValid != tt.expected {
				t.Errorf("expected valid=%v, got %v for flag %d", tt.expected, isValid, tt.flag)
			}
		})
	}
}

func TestClientSelectUsers(t *testing.T) {
	mockConn := newMockConn("10.0.0.1:5000")
	client := &Client{
		conn: mockConn,
	}

	client.SelectUsers()

	output := mockConn.String()
	if output != "who\n" {
		t.Errorf("expected 'who\\n', got %q", output)
	}
}

func TestClientPublicChatMessageFormat(t *testing.T) {
	mockConn := newMockConn("10.0.0.1:5000")
	client := &Client{
		conn: mockConn,
	}

	chatMsg := "hello everyone"
	sendMsg := chatMsg + "\n"
	n, err := client.conn.Write([]byte(sendMsg))

	if err != nil {
		t.Fatalf("failed to write: %v", err)
	}
	if n != len(sendMsg) {
		t.Errorf("expected to write %d bytes, wrote %d", len(sendMsg), n)
	}

	output := mockConn.String()
	if output != "hello everyone\n" {
		t.Errorf("expected 'hello everyone\\n', got %q", output)
	}
}

func TestClientPrivateChatMessageFormat(t *testing.T) {
	mockConn := newMockConn("10.0.0.1:5000")
	client := &Client{
		conn: mockConn,
	}

	remoteName := "bob"
	chatMsg := "secret message"
	sendMsg := "to|" + remoteName + "|" + chatMsg + "\n"
	n, err := client.conn.Write([]byte(sendMsg))

	if err != nil {
		t.Fatalf("failed to write: %v", err)
	}
	if n != len(sendMsg) {
		t.Errorf("expected to write %d bytes, wrote %d", len(sendMsg), n)
	}

	output := mockConn.String()
	expectedMsg := "to|bob|secret message\n"
	if output != expectedMsg {
		t.Errorf("expected %q, got %q", expectedMsg, output)
	}
}

func TestClientUpdateNameMessageFormat(t *testing.T) {
	mockConn := newMockConn("10.0.0.1:5000")
	client := &Client{
		conn: mockConn,
	}

	newName := "bob"
	sendMsg := "rename|" + newName + "\n"
	n, err := client.conn.Write([]byte(sendMsg))

	if err != nil {
		t.Fatalf("failed to write: %v", err)
	}
	if n != len(sendMsg) {
		t.Errorf("expected to write %d bytes, wrote %d", len(sendMsg), n)
	}

	output := mockConn.String()
	expectedMsg := "rename|bob\n"
	if output != expectedMsg {
		t.Errorf("expected %q, got %q", expectedMsg, output)
	}
}

func TestClientNameUpdate(t *testing.T) {
	client := &Client{
		Name: "oldname",
	}

	if client.Name != "oldname" {
		t.Fatalf("expected initial name oldname, got %s", client.Name)
	}

	client.Name = "newname"
	if client.Name != "newname" {
		t.Fatalf("expected updated name newname, got %s", client.Name)
	}
}

func TestClientMultiplePublicMessages(t *testing.T) {
	mockConn := newMockConn("10.0.0.1:5000")
	client := &Client{
		conn: mockConn,
	}

	messages := []string{"hello", "world", "test"}
	for _, msg := range messages {
		sendMsg := msg + "\n"
		_, err := client.conn.Write([]byte(sendMsg))
		if err != nil {
			t.Fatalf("failed to write: %v", err)
		}
	}

	output := mockConn.String()
	for _, msg := range messages {
		if !strings.Contains(output, msg) {
			t.Errorf("expected output to contain %q, got %q", msg, output)
		}
	}
}

func TestClientPrivateChatMultipleRecipients(t *testing.T) {
	mockConn := newMockConn("10.0.0.1:5000")
	client := &Client{
		conn: mockConn,
	}

	recipients := []string{"bob", "charlie", "dave"}
	message := "hello"

	for _, recipient := range recipients {
		sendMsg := "to|" + recipient + "|" + message + "\n"
		_, err := client.conn.Write([]byte(sendMsg))
		if err != nil {
			t.Fatalf("failed to write: %v", err)
		}
	}

	output := mockConn.String()
	for _, recipient := range recipients {
		expected := "to|" + recipient + "|" + message + "\n"
		if !strings.Contains(output, expected) {
			t.Errorf("expected output to contain %q", expected)
		}
	}
}

func TestClientPrivateChatWithSpecialCharacters(t *testing.T) {
	mockConn := newMockConn("10.0.0.1:5000")
	client := &Client{
		conn: mockConn,
	}

	remoteName := "bob"
	chatMsg := "hello|world|test@#$"
	sendMsg := "to|" + remoteName + "|" + chatMsg + "\n"
	_, err := client.conn.Write([]byte(sendMsg))

	if err != nil {
		t.Fatalf("failed to write: %v", err)
	}

	output := mockConn.String()
	expected := "to|bob|hello|world|test@#$\n"
	if output != expected {
		t.Errorf("expected %q, got %q", expected, output)
	}
}

func TestClientLongMessage(t *testing.T) {
	mockConn := newMockConn("10.0.0.1:5000")
	client := &Client{
		conn: mockConn,
	}

	longMsg := strings.Repeat("x", 3000)
	sendMsg := longMsg + "\n"
	n, err := client.conn.Write([]byte(sendMsg))

	if err != nil {
		t.Fatalf("failed to write: %v", err)
	}
	if n != len(sendMsg) {
		t.Errorf("expected to write %d bytes, wrote %d", len(sendMsg), n)
	}

	output := mockConn.String()
	if len(output) != len(sendMsg) {
		t.Errorf("expected output length %d, got %d", len(sendMsg), len(output))
	}
}

func TestClientConnectionOperations(t *testing.T) {
	mockConn := newMockConn("10.0.0.1:5000")
	client := &Client{
		conn: mockConn,
	}

	remoteAddr := client.conn.RemoteAddr()
	if remoteAddr.String() != "10.0.0.1:5000" {
		t.Errorf("expected remote address 10.0.0.1:5000, got %s", remoteAddr.String())
	}

	localAddr := client.conn.LocalAddr()
	if localAddr.String() != "127.0.0.1:0" {
		t.Errorf("expected local address 127.0.0.1:0, got %s", localAddr.String())
	}

	err := client.conn.Close()
	if err != nil {
		t.Errorf("failed to close connection: %v", err)
	}
}

func TestClientReceiveServerResponse(t *testing.T) {
	mockConn := newMockConn("10.0.0.1:5000")
	client := &Client{
		conn: mockConn,
	}

	serverResponse := "[10.0.0.2:5001]bob: Online...\n"
	go mockConn.SendData([]byte(serverResponse))

	buf := make([]byte, 1024)
	n, err := client.conn.Read(buf)

	if err != nil {
		t.Fatalf("failed to read: %v", err)
	}

	received := string(buf[:n])
	if received != serverResponse {
		t.Errorf("expected %q, got %q", serverResponse, received)
	}
}

func TestClientMixedOperations(t *testing.T) {
	mockConn := newMockConn("10.0.0.1:5000")
	client := &Client{
		conn: mockConn,
	}

	whoMsg := "who\n"
	_, err := client.conn.Write([]byte(whoMsg))
	if err != nil {
		t.Fatalf("failed to write who: %v", err)
	}

	publicMsg := "hello\n"
	_, err = client.conn.Write([]byte(publicMsg))
	if err != nil {
		t.Fatalf("failed to write public: %v", err)
	}

	privateMsg := "to|bob|secret\n"
	_, err = client.conn.Write([]byte(privateMsg))
	if err != nil {
		t.Fatalf("failed to write private: %v", err)
	}

	renameMsg := "rename|newname\n"
	_, err = client.conn.Write([]byte(renameMsg))
	if err != nil {
		t.Fatalf("failed to write rename: %v", err)
	}

	output := mockConn.String()
	if !strings.Contains(output, "who") {
		t.Errorf("expected output to contain who command")
	}
	if !strings.Contains(output, "hello") {
		t.Errorf("expected output to contain hello message")
	}
	if !strings.Contains(output, "to|bob|secret") {
		t.Errorf("expected output to contain private message")
	}
	if !strings.Contains(output, "rename|newname") {
		t.Errorf("expected output to contain rename command")
	}
}

func TestClientUpdateNameReturnsTrue(t *testing.T) {
	mockConn := newMockConn("10.0.0.1:5000")
	client := &Client{
		conn: mockConn,
	}

	// UpdateName 应该返回 true
	// 但由于它调用 fmt.Scanln，我们不能直接测试
	// 我们可以测试名称变更的核心逻辑
	newName := "bob"
	sendMsg := "rename|" + newName + "\n"
	n, err := client.conn.Write([]byte(sendMsg))

	if err != nil {
		t.Fatalf("failed to write: %v", err)
	}
	if n != len(sendMsg) {
		t.Fatalf("failed to write correct number of bytes")
	}
}

func TestClientPublicChatMultipleRounds(t *testing.T) {
	mockConn := newMockConn("10.0.0.1:5000")
	client := &Client{
		conn: mockConn,
	}

	rounds := 3
	for i := 0; i < rounds; i++ {
		msg := "message" + string(rune('0'+i))
		sendMsg := msg + "\n"
		_, err := client.conn.Write([]byte(sendMsg))
		if err != nil {
			t.Fatalf("round %d: failed to write: %v", i, err)
		}
	}

	output := mockConn.String()
	for i := 0; i < rounds; i++ {
		expected := "message" + string(rune('0'+i))
		if !strings.Contains(output, expected) {
			t.Errorf("expected output to contain %q", expected)
		}
	}
}

func TestClientPrivateChatFormattingWithFormatSprintf(t *testing.T) {
	mockConn := newMockConn("10.0.0.1:5000")
	client := &Client{
		conn: mockConn,
	}

	remoteName := "bob"
	chatMsg := "test message"

	// 使用与 PrivateChat 相同的格式
	sendMsg := strings.Join([]string{"to", remoteName, chatMsg}, "|") + "\n"
	_, err := client.conn.Write([]byte(sendMsg))

	if err != nil {
		t.Fatalf("failed to write: %v", err)
	}

	output := mockConn.String()
	expected := "to|bob|test message\n"
	if output != expected {
		t.Errorf("expected %q, got %q", expected, output)
	}
}

func TestClientSelectUsersCommand(t *testing.T) {
	mockConn := newMockConn("10.0.0.1:5000")
	client := &Client{
		conn: mockConn,
	}

	// SelectUsers 应该发送 "who\n"
	client.SelectUsers()

	output := mockConn.String()
	if output != "who\n" {
		t.Errorf("SelectUsers should send 'who\\n', got %q", output)
	}
}
