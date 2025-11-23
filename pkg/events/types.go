package events

import "time"

type EventType uint8

const (
	EventTypeExec     EventType = 1
	EventTypeFileOpen EventType = 2

	// Buffer sizes (must match BPF definitions)
	TaskCommLen = 16
	PathMaxLen  = 256
)

type ExecEvent struct {
	PID      uint32
	PPID     uint32
	CgroupID uint64
	Comm     [TaskCommLen]byte
	PComm    [TaskCommLen]byte
}

type FileOpenEvent struct {
	PID      uint32
	CgroupID uint64
	Flags    uint32
	Filename [PathMaxLen]byte
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
