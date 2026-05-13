package handler

import (
	"sync"

	"gopher-mock/model"
)

// LogStore is a thread-safe in-memory ring buffer for request logs.
// It keeps at most maxSize entries and broadcasts to SSE subscribers.
type LogStore struct {
	mu      sync.RWMutex
	entries []model.RequestLog
	maxSize int
	subs    map[chan model.RequestLog]struct{}
}

// NewLogStore creates a LogStore with a given capacity.
func NewLogStore(maxSize int) *LogStore {
	return &LogStore{
		entries: make([]model.RequestLog, 0, maxSize),
		maxSize: maxSize,
		subs:    make(map[chan model.RequestLog]struct{}),
	}
}

// Add appends an entry (evicting oldest if at capacity) and broadcasts to SSE subs.
func (s *LogStore) Add(entry model.RequestLog) {
	s.mu.Lock()
	s.entries = append(s.entries, entry)
	if len(s.entries) > s.maxSize {
		s.entries = s.entries[len(s.entries)-s.maxSize:]
	}
	subs := make([]chan model.RequestLog, 0, len(s.subs))
	for ch := range s.subs {
		subs = append(subs, ch)
	}
	s.mu.Unlock()

	for _, ch := range subs {
		select {
		case ch <- entry:
		default: // drop if subscriber is slow
		}
	}
}

// GetAll returns a snapshot of all current entries.
func (s *LogStore) GetAll() []model.RequestLog {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]model.RequestLog, len(s.entries))
	copy(result, s.entries)
	return result
}

// Clear removes all log entries.
func (s *LogStore) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.entries = s.entries[:0]
}

// Count returns number of stored entries.
func (s *LogStore) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.entries)
}

// Subscribe returns a channel that receives new log entries in real time.
func (s *LogStore) Subscribe() chan model.RequestLog {
	ch := make(chan model.RequestLog, 64)
	s.mu.Lock()
	s.subs[ch] = struct{}{}
	s.mu.Unlock()
	return ch
}

// Unsubscribe removes and closes a subscriber channel.
func (s *LogStore) Unsubscribe(ch chan model.RequestLog) {
	s.mu.Lock()
	delete(s.subs, ch)
	s.mu.Unlock()
	close(ch)
}
