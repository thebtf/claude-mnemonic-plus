package worker

import (
	"bufio"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/thebtf/engram/internal/logbuf"
)

func newTestServiceWithLogBuffer() *Service {
	buf := logbuf.NewRingBuffer(100)
	return &Service{
		logBuffer: buf,
	}
}

func writeTestLog(buf *logbuf.RingBuffer, level, message string) {
	entry := map[string]any{
		"time":    time.Now().Unix(),
		"level":   level,
		"message": message,
	}
	data, _ := json.Marshal(entry)
	data = append(data, '\n')
	buf.Write(data)
}

func TestHandleGetLogs_Snapshot(t *testing.T) {
	svc := newTestServiceWithLogBuffer()
	writeTestLog(svc.logBuffer, "info", "test message 1")
	writeTestLog(svc.logBuffer, "error", "test message 2")

	req := httptest.NewRequest("GET", "/api/logs", nil)
	w := httptest.NewRecorder()
	svc.handleGetLogs(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("expected application/json, got %q", ct)
	}

	var entries []logbuf.LogEntry
	if err := json.NewDecoder(w.Body).Decode(&entries); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
}

func TestHandleGetLogs_LevelFilter(t *testing.T) {
	svc := newTestServiceWithLogBuffer()
	writeTestLog(svc.logBuffer, "debug", "debug msg")
	writeTestLog(svc.logBuffer, "info", "info msg")
	writeTestLog(svc.logBuffer, "error", "error msg")

	req := httptest.NewRequest("GET", "/api/logs?level=error", nil)
	w := httptest.NewRecorder()
	svc.handleGetLogs(w, req)

	var entries []logbuf.LogEntry
	json.NewDecoder(w.Body).Decode(&entries)
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Level != "error" {
		t.Errorf("expected error level, got %q", entries[0].Level)
	}
}

func TestHandleGetLogs_QueryFilter(t *testing.T) {
	svc := newTestServiceWithLogBuffer()
	writeTestLog(svc.logBuffer, "info", "starting server")
	writeTestLog(svc.logBuffer, "info", "database ready")

	req := httptest.NewRequest("GET", "/api/logs?query=server", nil)
	w := httptest.NewRecorder()
	svc.handleGetLogs(w, req)

	var entries []logbuf.LogEntry
	json.NewDecoder(w.Body).Decode(&entries)
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
}

func TestHandleGetLogs_LinesParam(t *testing.T) {
	svc := newTestServiceWithLogBuffer()
	for i := 0; i < 10; i++ {
		writeTestLog(svc.logBuffer, "info", "msg")
	}

	req := httptest.NewRequest("GET", "/api/logs?lines=3", nil)
	w := httptest.NewRecorder()
	svc.handleGetLogs(w, req)

	var entries []logbuf.LogEntry
	json.NewDecoder(w.Body).Decode(&entries)
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}
}

func TestHandleGetLogs_EmptyBuffer(t *testing.T) {
	svc := newTestServiceWithLogBuffer()

	req := httptest.NewRequest("GET", "/api/logs", nil)
	w := httptest.NewRecorder()
	svc.handleGetLogs(w, req)

	var entries []logbuf.LogEntry
	json.NewDecoder(w.Body).Decode(&entries)
	if len(entries) != 0 {
		t.Fatalf("expected 0 entries, got %d", len(entries))
	}
}

func TestHandleGetLogs_NilBuffer(t *testing.T) {
	svc := &Service{}
	req := httptest.NewRequest("GET", "/api/logs", nil)
	w := httptest.NewRecorder()
	svc.handleGetLogs(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", w.Code)
	}
}

func TestHandleGetLogs_SSEFollow(t *testing.T) {
	svc := newTestServiceWithLogBuffer()

	ctx, cancel := context.WithCancel(context.Background())
	req := httptest.NewRequest("GET", "/api/logs?follow=true", nil).WithContext(ctx)
	w := httptest.NewRecorder()

	done := make(chan struct{})
	go func() {
		svc.handleGetLogs(w, req)
		close(done)
	}()

	// Give handler time to set up
	time.Sleep(50 * time.Millisecond)

	// Write a log entry
	writeTestLog(svc.logBuffer, "info", "streamed message")

	// Give it time to be written
	time.Sleep(50 * time.Millisecond)

	// Cancel context to stop SSE
	cancel()
	<-done

	body := w.Body.String()
	if !strings.Contains(body, "streamed message") {
		t.Errorf("expected SSE body to contain 'streamed message', got: %s", body)
	}

	// Verify SSE format
	scanner := bufio.NewScanner(strings.NewReader(body))
	foundData := false
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "data: ") {
			foundData = true
			var entry logbuf.LogEntry
			if err := json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &entry); err != nil {
				t.Errorf("failed to parse SSE data as LogEntry: %v", err)
			}
		}
	}
	if !foundData {
		t.Error("expected at least one SSE data line")
	}
}
