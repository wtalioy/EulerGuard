package storage

import (
	"sync"
	"time"

	"aegis/pkg/events"
)

// TimeRingBuffer implements EventStore using a ring buffer with time-based indexing.
type TimeRingBuffer struct {
	events    []*Event
	capacity  int
	writePos  int64 // Atomic counter for write position
	mu        sync.RWMutex
	startTime time.Time // Time of first event
}

// NewTimeRingBuffer creates a new TimeRingBuffer with the specified capacity.
func NewTimeRingBuffer(capacity int) *TimeRingBuffer {
	if capacity <= 0 {
		capacity = 10000 // Default capacity
	}
	return &TimeRingBuffer{
		events:   make([]*Event, capacity),
		capacity: capacity,
	}
}

// Append adds an event to the ring buffer.
func (rb *TimeRingBuffer) Append(event *Event) error {
	if event == nil {
		return nil
	}

	// Set timestamp if not set
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}

	rb.mu.Lock()
	defer rb.mu.Unlock()

	// Set start time on first event
	if rb.startTime.IsZero() {
		rb.startTime = event.Timestamp
	}

	// Calculate position using atomic counter
	pos := int(rb.writePos % int64(rb.capacity))
	rb.events[pos] = event
	rb.writePos++

	return nil
}

// Query returns events within the given time range.
func (rb *TimeRingBuffer) Query(start, end time.Time) ([]*Event, error) {
	rb.mu.RLock()
	defer rb.mu.RUnlock()

	if rb.writePos == 0 {
		return []*Event{}, nil
	}

	var results []*Event
	totalEvents := int(rb.writePos)
	if totalEvents > rb.capacity {
		totalEvents = rb.capacity
	}

	// Start from the oldest event (writePos - totalEvents) or 0
	startIdx := 0
	if totalEvents == rb.capacity {
		// Buffer is full, start from the oldest event
		oldestPos := int(rb.writePos % int64(rb.capacity))
		startIdx = oldestPos
	}

	// Iterate through events in chronological order
	for i := 0; i < totalEvents; i++ {
		idx := (startIdx + i) % rb.capacity
		event := rb.events[idx]
		if event == nil {
			continue
		}

		// Check if event is within time range
		if !event.Timestamp.Before(start) && !event.Timestamp.After(end) {
			results = append(results, event)
		}
	}

	return results, nil
}

// Latest returns the most recent N events.
func (rb *TimeRingBuffer) Latest(n int) ([]*Event, error) {
	rb.mu.RLock()
	defer rb.mu.RUnlock()

	if n <= 0 {
		return []*Event{}, nil
	}

	if rb.writePos == 0 {
		return []*Event{}, nil
	}

	totalEvents := int(rb.writePos)
	if totalEvents > rb.capacity {
		totalEvents = rb.capacity
	}

	if n > totalEvents {
		n = totalEvents
	}

	results := make([]*Event, 0, n)

	// Start from the most recent event and work backwards
	startPos := int((rb.writePos - 1) % int64(rb.capacity))
	for i := 0; i < n; i++ {
		idx := (startPos - i + rb.capacity) % rb.capacity
		event := rb.events[idx]
		if event != nil {
			// Insert at beginning to maintain chronological order
			results = append([]*Event{event}, results...)
		}
	}

	return results, nil
}

// Close closes the ring buffer and releases resources.
func (rb *TimeRingBuffer) Close() error {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	rb.events = nil
	return nil
}

// Size returns the current number of events in the buffer.
func (rb *TimeRingBuffer) Size() int {
	rb.mu.RLock()
	defer rb.mu.RUnlock()
	totalEvents := int(rb.writePos)
	if totalEvents > rb.capacity {
		return rb.capacity
	}
	return totalEvents
}

// Capacity returns the maximum capacity of the ring buffer.
func (rb *TimeRingBuffer) Capacity() int {
	return rb.capacity
}

// EventFromBackend creates an Event from backend event types.
func EventFromBackend(eventType events.EventType, timestamp time.Time, data interface{}) *Event {
	return &Event{
		Type:      eventType,
		Timestamp: timestamp,
		Data:      data,
	}
}
