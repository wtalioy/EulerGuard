package storage

import (
	"sync"
	"time"

	"aegis/pkg/events"
)

// Manager combines TimeRingBuffer and Indexer for unified event storage and querying.
type Manager struct {
	store   *TimeRingBuffer
	indexer *Indexer
	mu      sync.RWMutex
}

// NewManager creates a new storage manager with the specified capacity.
func NewManager(capacity int, maxIndexSize int) *Manager {
	return &Manager{
		store:   NewTimeRingBuffer(capacity),
		indexer: NewIndexer(maxIndexSize),
	}
}

// Append adds an event to both the ring buffer and indexer.
func (m *Manager) Append(event *Event) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if err := m.store.Append(event); err != nil {
		return err
	}

	m.indexer.IndexEvent(event)
	return nil
}

// Query returns events within the given time range.
func (m *Manager) Query(start, end time.Time) ([]*Event, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.store.Query(start, end)
}

// Latest returns the most recent N events.
func (m *Manager) Latest(n int) ([]*Event, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.store.Latest(n)
}

// QueryByPID returns all events for a given PID.
func (m *Manager) QueryByPID(pid uint32) []*Event {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.indexer.QueryByPID(pid)
}

// QueryByCgroup returns all events for a given CgroupID.
func (m *Manager) QueryByCgroup(cgroupID uint64) []*Event {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.indexer.QueryByCgroup(cgroupID)
}

// QueryByType returns all events of a given type.
func (m *Manager) QueryByType(eventType events.EventType) []*Event {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.indexer.QueryByType(eventType)
}

// QueryByProcess returns all events for a given process name.
func (m *Manager) QueryByProcess(processName string) []*Event {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.indexer.QueryByProcess(processName)
}

// QueryByFilter performs a combination query using the filter.
func (m *Manager) QueryByFilter(filter Filter) []*Event {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.indexer.QueryByFilter(filter)
}

// Close closes the storage manager.
func (m *Manager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.store.Close()
}

// Size returns the current number of events in the buffer.
func (m *Manager) Size() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.store.Size()
}

// Capacity returns the maximum capacity of the ring buffer.
func (m *Manager) Capacity() int {
	return m.store.Capacity()
}
