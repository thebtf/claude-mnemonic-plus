// Package logbuf provides an in-memory ring buffer for structured log entries.
// It implements io.Writer for use with zerolog.MultiLevelWriter.
package logbuf

import (
	"encoding/json"
	"strings"
	"sync"
	"time"
)

// DefaultCapacity is the default number of log entries the ring buffer holds.
const DefaultCapacity = 10000

// LogEntry represents a single structured log entry.
type LogEntry struct {
	Timestamp int64          `json:"timestamp"`
	Level     string         `json:"level"`
	Message   string         `json:"message"`
	Fields    map[string]any `json:"fields,omitempty"`
	Raw       string         `json:"raw"`
}

// RingBuffer is a thread-safe circular buffer of log entries with pub/sub support.
type RingBuffer struct {
	entries     []LogEntry
	subscribers []chan LogEntry
	mu          sync.Mutex
	head        int
	size        int
	capacity    int
}

// NewRingBuffer creates a new ring buffer with the given capacity.
// If capacity <= 0, DefaultCapacity is used.
func NewRingBuffer(capacity int) *RingBuffer {
	if capacity <= 0 {
		capacity = DefaultCapacity
	}
	return &RingBuffer{
		entries:  make([]LogEntry, capacity),
		capacity: capacity,
	}
}

// Write implements io.Writer for zerolog. Each call receives one complete JSON log line.
// It parses the JSON into a LogEntry and appends it to the ring buffer.
func (rb *RingBuffer) Write(p []byte) (int, error) {
	raw := strings.TrimSpace(string(p))
	if raw == "" {
		return len(p), nil
	}

	entry := LogEntry{Raw: raw}

	// Parse structured fields from the JSON log line
	var parsed map[string]any
	if err := json.Unmarshal([]byte(raw), &parsed); err == nil {
		if v, ok := parsed["time"]; ok {
			switch t := v.(type) {
			case float64:
				entry.Timestamp = int64(t)
			case string:
				if ts, err := time.Parse(time.RFC3339, t); err == nil {
					entry.Timestamp = ts.Unix()
				}
			}
		}
		if v, ok := parsed["level"].(string); ok {
			entry.Level = v
		}
		if v, ok := parsed["message"].(string); ok {
			entry.Message = v
		}

		// Collect remaining fields (exclude standard zerolog keys)
		fields := make(map[string]any)
		for k, v := range parsed {
			switch k {
			case "time", "level", "message":
				continue
			default:
				fields[k] = v
			}
		}
		if len(fields) > 0 {
			entry.Fields = fields
		}
	} else {
		// Non-JSON line: store as message
		entry.Message = raw
		entry.Timestamp = time.Now().Unix()
	}

	rb.mu.Lock()
	rb.entries[rb.head] = entry
	rb.head = (rb.head + 1) % rb.capacity
	if rb.size < rb.capacity {
		rb.size++
	}

	// Notify subscribers (non-blocking)
	for _, ch := range rb.subscribers {
		select {
		case ch <- entry:
		default:
			// Drop if subscriber is slow
		}
	}
	rb.mu.Unlock()

	return len(p), nil
}

// Snapshot returns the last n entries, optionally filtered by minimum level and query string.
// If n <= 0 or n > available entries, all available entries are returned.
// Entries are returned in chronological order (oldest first).
func (rb *RingBuffer) Snapshot(n int, minLevel string, query string) []LogEntry {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	if rb.size == 0 {
		return nil
	}

	// Collect all entries in chronological order
	all := make([]LogEntry, 0, rb.size)
	start := (rb.head - rb.size + rb.capacity) % rb.capacity
	for i := 0; i < rb.size; i++ {
		idx := (start + i) % rb.capacity
		e := rb.entries[idx]

		if minLevel != "" && !LevelAtLeast(e.Level, minLevel) {
			continue
		}
		if query != "" && !EntryMatchesQuery(e, query) {
			continue
		}
		all = append(all, e)
	}

	if n <= 0 || n > len(all) {
		return all
	}
	return all[len(all)-n:]
}

// Subscribe returns a channel that receives new log entries and an unsubscribe function.
// The channel has a buffer of 64 entries; slow consumers will miss entries.
func (rb *RingBuffer) Subscribe() (chan LogEntry, func()) {
	ch := make(chan LogEntry, 64)

	rb.mu.Lock()
	rb.subscribers = append(rb.subscribers, ch)
	rb.mu.Unlock()

	unsubscribe := func() {
		rb.mu.Lock()
		defer rb.mu.Unlock()
		for i, sub := range rb.subscribers {
			if sub == ch {
				rb.subscribers = append(rb.subscribers[:i], rb.subscribers[i+1:]...)
				close(ch)
				return
			}
		}
	}

	return ch, unsubscribe
}

// levelPriority maps zerolog level strings to numeric priority.
var levelPriority = map[string]int{
	"trace": 0,
	"debug": 1,
	"info":  2,
	"warn":  3,
	"error": 4,
	"fatal": 5,
	"panic": 6,
}

// LevelAtLeast returns true if entryLevel is at least as severe as minLevel.
func LevelAtLeast(entryLevel, minLevel string) bool {
	ep, ok1 := levelPriority[entryLevel]
	mp, ok2 := levelPriority[minLevel]
	if !ok1 || !ok2 {
		return true // Unknown levels pass through
	}
	return ep >= mp
}

// EntryMatchesQuery returns true if any of the entry's text fields contain the query (case-insensitive).
func EntryMatchesQuery(e LogEntry, query string) bool {
	q := strings.ToLower(query)
	if strings.Contains(strings.ToLower(e.Message), q) {
		return true
	}
	if strings.Contains(strings.ToLower(e.Raw), q) {
		return true
	}
	return false
}
