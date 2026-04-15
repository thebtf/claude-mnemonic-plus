// Package control — integration tests for the control socket listener.
//
// Tests cover T060 (simplified integration test):
//   - Command protocol: graceful-restart → ACK
//   - Unknown command → ERR
//   - Socket file cleanup on Close
//
// These tests run on all platforms via net.Listen (TCP fallback in tests).
// The Unix domain socket path is exercised only on unix-tagged builds.
package control

import (
	"bufio"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// newTestListener creates a Listener backed by an in-process net.Listener
// rather than a real Unix socket. This lets the protocol tests run on all
// platforms without needing filesystem socket support.
func newTestListenerOnTCP(t *testing.T, handler CommandHandler) (*Listener, string) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen: %v", err)
	}
	addr := ln.Addr().String()

	l := &Listener{
		socketPath: addr, // not a real path — just for Close log
		pidPath:    filepath.Join(t.TempDir(), "engram.pid"),
		handler:    handler,
		logger:     slog.Default(),
		ln:         ln,
	}
	go l.serve()
	t.Cleanup(func() { _ = ln.Close() })
	return l, addr
}

// dialAndExchange connects to addr, sends cmd+"\n", and returns the response
// line (without trailing "\n").
func dialAndExchange(t *testing.T, addr, cmd string) string {
	t.Helper()
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("dial %s: %v", addr, err)
	}
	defer conn.Close()

	if _, err := conn.Write([]byte(cmd + "\n")); err != nil {
		t.Fatalf("write: %v", err)
	}

	scanner := bufio.NewScanner(conn)
	if !scanner.Scan() {
		t.Fatalf("no response line")
	}
	return scanner.Text()
}

// TestControlSocket_GracefulRestart asserts that the "graceful-restart"
// command receives an "ACK" response.
func TestControlSocket_GracefulRestart(t *testing.T) {
	ackReceived := false
	handler := func(cmd string) string {
		if cmd == "graceful-restart" {
			ackReceived = true
			return "ACK"
		}
		return "ERR unknown command"
	}

	_, addr := newTestListenerOnTCP(t, handler)
	got := dialAndExchange(t, addr, "graceful-restart")
	if got != "ACK" {
		t.Errorf("graceful-restart: want ACK, got %q", got)
	}
	if !ackReceived {
		t.Error("handler was not called for graceful-restart")
	}
}

// TestControlSocket_UnknownCommand asserts that unknown commands receive an
// "ERR" prefixed response.
func TestControlSocket_UnknownCommand(t *testing.T) {
	handler := func(cmd string) string {
		if cmd == "graceful-restart" {
			return "ACK"
		}
		return "ERR unknown command"
	}

	_, addr := newTestListenerOnTCP(t, handler)
	got := dialAndExchange(t, addr, "reboot-everything")
	if !strings.HasPrefix(got, "ERR") {
		t.Errorf("unknown command: want ERR prefix, got %q", got)
	}
}

// TestControlSocket_MultipleCommands verifies that multiple sequential
// connections each get independent responses.
func TestControlSocket_MultipleCommands(t *testing.T) {
	calls := 0
	handler := func(cmd string) string {
		calls++
		return "ACK"
	}

	_, addr := newTestListenerOnTCP(t, handler)

	for i := 0; i < 3; i++ {
		got := dialAndExchange(t, addr, "graceful-restart")
		if got != "ACK" {
			t.Errorf("call %d: want ACK, got %q", i, got)
		}
	}
	if calls != 3 {
		t.Errorf("expected 3 handler calls, got %d", calls)
	}
}

// TestControlSocket_SocketFileCleanup verifies that Close removes the socket
// file and PID file. Uses a real temp-dir path (not an actual socket bind —
// we just test the cleanup logic by pre-creating the files manually).
func TestControlSocket_SocketFileCleanup(t *testing.T) {
	dir := t.TempDir()
	sockPath := filepath.Join(dir, "test.sock")
	pidPath := filepath.Join(dir, "test.pid")

	// Pre-create the files to simulate a running listener's artifacts.
	if err := os.WriteFile(sockPath, []byte{}, 0o600); err != nil {
		t.Fatalf("create test sock file: %v", err)
	}
	if err := os.WriteFile(pidPath, []byte("12345\n"), 0o644); err != nil {
		t.Fatalf("create test pid file: %v", err)
	}

	l := &Listener{
		socketPath: sockPath,
		pidPath:    pidPath,
		logger:     slog.Default(),
	}
	// No real net.Listener — Close should still remove the files.
	l.Close()

	if _, err := os.Stat(sockPath); !os.IsNotExist(err) {
		t.Error("socket file was not removed by Close")
	}
	if _, err := os.Stat(pidPath); !os.IsNotExist(err) {
		t.Error("pid file was not removed by Close")
	}
}
