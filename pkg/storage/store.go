package storage

import (
	"time"

	"aegis/pkg/events"
)

// Event represents a stored event with metadata.
type Event struct {
	Type      events.EventType
	Timestamp time.Time
	Data      interface{} // Can be *events.ExecEvent, *events.FileOpenEvent, or *events.ConnectEvent
}

// EventStore defines the interface for event storage.
// Phase 1 will implement this with TimeRingBuffer.
type EventStore interface {
	// Append adds an event to the store.
	Append(event *Event) error

	// Query returns events within the given time range.
	Query(start, end time.Time) ([]*Event, error)

	// Latest returns the most recent N events.
	Latest(n int) ([]*Event, error)

	// Close closes the store and releases resources.
	Close() error
}

// Filter represents a query filter for events.
type Filter struct {
	Types     []events.EventType
	PIDs      []uint32
	CgroupIDs []uint64
	Processes []string
}
