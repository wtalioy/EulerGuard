package events

import "time"

type EventType uint8

const (
	EventTypeExec     EventType = 1
	EventTypeFileOpen EventType = 2
)

type ExecEvent struct {
	PID      uint32
	PPID     uint32
	CgroupID uint64
	Comm     [16]byte
	PComm    [16]byte
}

type FileOpenEvent struct {
	PID      uint32
	CgroupID uint64
	Flags    uint32
	Filename [256]byte
}

type Event struct {
	Type     EventType
	Exec     *ExecEvent
	FileOpen *FileOpenEvent
}

type ProcessedEvent struct {
	Event     ExecEvent
	Timestamp time.Time
	Process   string
	Parent    string
	Rate      float64
}
