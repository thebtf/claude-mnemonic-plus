package logbuf

import (
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"
)

func writeJSON(rb *RingBuffer, level, message string) {
	entry := map[string]any{
		"time":    time.Now().Unix(),
		"level":   level,
		"message": message,
	}
	data, _ := json.Marshal(entry)
	data = append(data, '\n')
	rb.Write(data)
}

func TestWriteAndSnapshot(t *testing.T) {
	rb := NewRingBuffer(100)

	writeJSON(rb, "info", "hello")
	writeJSON(rb, "error", "world")

	entries := rb.Snapshot(0, "", "")
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	if entries[0].Message != "hello" {
		t.Errorf("expected 'hello', got %q", entries[0].Message)
	}
	if entries[1].Level != "error" {
		t.Errorf("expected 'error', got %q", entries[1].Level)
	}
}

func TestSnapshotLimit(t *testing.T) {
	rb := NewRingBuffer(100)
	for i := 0; i < 10; i++ {
		writeJSON(rb, "info", fmt.Sprintf("msg-%d", i))
	}

	entries := rb.Snapshot(3, "", "")
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}
	// Should be the last 3
	if entries[0].Message != "msg-7" {
		t.Errorf("expected 'msg-7', got %q", entries[0].Message)
	}
}

func TestOverflow(t *testing.T) {
	rb := NewRingBuffer(5)
	for i := 0; i < 8; i++ {
		writeJSON(rb, "info", fmt.Sprintf("msg-%d", i))
	}

	entries := rb.Snapshot(0, "", "")
	if len(entries) != 5 {
		t.Fatalf("expected 5 entries (capacity), got %d", len(entries))
	}
	// Oldest should be msg-3 (first 3 overwritten)
	if entries[0].Message != "msg-3" {
		t.Errorf("expected 'msg-3', got %q", entries[0].Message)
	}
	if entries[4].Message != "msg-7" {
		t.Errorf("expected 'msg-7', got %q", entries[4].Message)
	}
}

func TestFilterByLevel(t *testing.T) {
	rb := NewRingBuffer(100)
	writeJSON(rb, "debug", "debug-msg")
	writeJSON(rb, "info", "info-msg")
	writeJSON(rb, "warn", "warn-msg")
	writeJSON(rb, "error", "error-msg")

	entries := rb.Snapshot(0, "warn", "")
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries (warn+error), got %d", len(entries))
	}
	if entries[0].Level != "warn" {
		t.Errorf("expected 'warn', got %q", entries[0].Level)
	}
	if entries[1].Level != "error" {
		t.Errorf("expected 'error', got %q", entries[1].Level)
	}
}

func TestFilterByQuery(t *testing.T) {
	rb := NewRingBuffer(100)
	writeJSON(rb, "info", "starting server")
	writeJSON(rb, "info", "database connected")
	writeJSON(rb, "error", "server error occurred")

	entries := rb.Snapshot(0, "", "server")
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries matching 'server', got %d", len(entries))
	}
}

func TestFilterByLevelAndQuery(t *testing.T) {
	rb := NewRingBuffer(100)
	writeJSON(rb, "info", "server started")
	writeJSON(rb, "error", "server crashed")
	writeJSON(rb, "error", "database error")

	entries := rb.Snapshot(0, "error", "server")
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Message != "server crashed" {
		t.Errorf("expected 'server crashed', got %q", entries[0].Message)
	}
}

func TestSubscribeAndUnsubscribe(t *testing.T) {
	rb := NewRingBuffer(100)

	ch, unsub := rb.Subscribe()

	writeJSON(rb, "info", "test-msg")

	select {
	case entry := <-ch:
		if entry.Message != "test-msg" {
			t.Errorf("expected 'test-msg', got %q", entry.Message)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for subscriber notification")
	}

	unsub()

	// Channel should be closed after unsubscribe
	_, ok := <-ch
	if ok {
		t.Error("expected channel to be closed after unsubscribe")
	}
}

func TestConcurrentWriteSnapshot(t *testing.T) {
	rb := NewRingBuffer(100)

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				writeJSON(rb, "info", fmt.Sprintf("goroutine-%d-msg-%d", n, j))
			}
		}(i)
	}

	// Read concurrently
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 50; i++ {
			rb.Snapshot(10, "", "")
		}
	}()

	wg.Wait()

	entries := rb.Snapshot(0, "", "")
	if len(entries) != 100 {
		t.Errorf("expected 100 entries (capacity), got %d", len(entries))
	}
}

func TestDefaultCapacity(t *testing.T) {
	rb := NewRingBuffer(0)
	if rb.capacity != DefaultCapacity {
		t.Errorf("expected default capacity %d, got %d", DefaultCapacity, rb.capacity)
	}

	rb2 := NewRingBuffer(-1)
	if rb2.capacity != DefaultCapacity {
		t.Errorf("expected default capacity %d, got %d", DefaultCapacity, rb2.capacity)
	}
}

func TestEmptySnapshot(t *testing.T) {
	rb := NewRingBuffer(100)
	entries := rb.Snapshot(0, "", "")
	if entries != nil {
		t.Errorf("expected nil for empty buffer, got %v", entries)
	}
}

func TestFieldsParsing(t *testing.T) {
	rb := NewRingBuffer(100)
	entry := map[string]any{
		"time":    time.Now().Unix(),
		"level":   "info",
		"message": "test",
		"version": "1.0",
		"port":    37777.0,
	}
	data, _ := json.Marshal(entry)
	rb.Write(data)

	entries := rb.Snapshot(0, "", "")
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Fields["version"] != "1.0" {
		t.Errorf("expected version field '1.0', got %v", entries[0].Fields["version"])
	}
}
