package events

import "time"

type ExecEvent struct {
	PID  uint32
	PPID uint32
}

type ProcessedEvent struct {
	Event     ExecEvent
	Timestamp time.Time
	Process   string
	Parent    string
	Rate      float64
}