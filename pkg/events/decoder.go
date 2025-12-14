package events

import (
	"encoding/binary"
	"fmt"
	"time"
)

const (
	// Event sizes with new unified header
	ExecEventSize     = EventHeaderSize + 4 + 4 + TaskCommLen + PathMaxLen + CommandLineLen // 56 + 4 + 4 + 16 + 256 + 512 = 848
	FileOpenEventSize = EventHeaderSize + 8 + 8 + 4 + 4 + PathMaxLen                  // 56 + 8 + 8 + 4 + 4 + 256 = 336
	ConnectEventSize  = EventHeaderSize + 4 + 2 + 2 + 16                              // 56 + 4 + 2 + 2 + 16 = 80
)

// DecodeHeader decodes the unified event header from raw data.
func DecodeHeader(data []byte) (EventHeader, error) {
	if len(data) < EventHeaderSize {
		return EventHeader{}, fmt.Errorf("event header too small: %d bytes", len(data))
	}

	var hdr EventHeader
	offset := 0
	hdr.TimestampNs = binary.LittleEndian.Uint64(data[offset : offset+8])
	offset += 8
	hdr.CgroupID = binary.LittleEndian.Uint64(data[offset : offset+8])
	offset += 8
	hdr.PID = binary.LittleEndian.Uint32(data[offset : offset+4])
	offset += 4
	hdr.TID = binary.LittleEndian.Uint32(data[offset : offset+4])
	offset += 4
	hdr.UID = binary.LittleEndian.Uint32(data[offset : offset+4])
	offset += 4
	hdr.GID = binary.LittleEndian.Uint32(data[offset : offset+4])
	offset += 4
	hdr.Type = EventType(data[offset])
	offset += 1
	hdr.Blocked = data[offset]
	offset += 7 // skip padding
	copy(hdr.Comm[:], data[offset:offset+TaskCommLen])

	return hdr, nil
}

// DecodeExecEvent decodes an exec event with the new unified header format.
func DecodeExecEvent(data []byte) (ExecEvent, error) {
	if len(data) < ExecEventSize {
		return ExecEvent{}, fmt.Errorf("exec event payload too small: %d bytes, expected %d", len(data), ExecEventSize)
	}

	var ev ExecEvent
	offset := 0

	// Decode header
	hdr, err := DecodeHeader(data[offset:])
	if err != nil {
		return ExecEvent{}, fmt.Errorf("decode header: %w", err)
	}
	ev.Hdr = hdr
	offset += EventHeaderSize

	// Decode exec-specific fields
	ev.PPID = binary.LittleEndian.Uint32(data[offset : offset+4])
	offset += 8 // skip padding
	copy(ev.PComm[:], data[offset:offset+TaskCommLen])
	offset += TaskCommLen
	copy(ev.Filename[:], data[offset:offset+PathMaxLen])
	offset += PathMaxLen
	
	// Read command_line
	copy(ev.CommandLine[:], data[offset:offset+CommandLineLen])

	return ev, nil
}

// DecodeFileOpenEvent decodes a file open event with the new unified header format.
func DecodeFileOpenEvent(data []byte) (FileOpenEvent, error) {
	if len(data) < FileOpenEventSize {
		return FileOpenEvent{}, fmt.Errorf("file open event too small: %d bytes, expected %d", len(data), FileOpenEventSize)
	}

	var ev FileOpenEvent
	offset := 0

	// Decode header
	hdr, err := DecodeHeader(data[offset:])
	if err != nil {
		return FileOpenEvent{}, fmt.Errorf("decode header: %w", err)
	}
	ev.Hdr = hdr
	offset += EventHeaderSize

	// Decode file-specific fields
	ev.Ino = binary.LittleEndian.Uint64(data[offset : offset+8])
	offset += 8
	ev.Dev = binary.LittleEndian.Uint64(data[offset : offset+8])
	offset += 8
	ev.Flags = binary.LittleEndian.Uint32(data[offset : offset+4])
	offset += 8 // skip padding
	copy(ev.Filename[:], data[offset:offset+PathMaxLen])

	return ev, nil
}

// DecodeConnectEvent decodes a connect event with the new unified header format.
func DecodeConnectEvent(data []byte) (ConnectEvent, error) {
	if len(data) < ConnectEventSize {
		return ConnectEvent{}, fmt.Errorf("connect event too small: %d bytes, expected %d", len(data), ConnectEventSize)
	}

	var ev ConnectEvent
	offset := 0

	// Decode header
	hdr, err := DecodeHeader(data[offset:])
	if err != nil {
		return ConnectEvent{}, fmt.Errorf("decode header: %w", err)
	}
	ev.Hdr = hdr
	offset += EventHeaderSize

	// Decode connect-specific fields
	ev.AddrV4 = binary.LittleEndian.Uint32(data[offset : offset+4])
	offset += 4
	ev.Family = binary.LittleEndian.Uint16(data[offset : offset+2])
	offset += 2
	ev.Port = binary.LittleEndian.Uint16(data[offset : offset+2])
	offset += 2
	copy(ev.AddrV6[:], data[offset:offset+16])

	return ev, nil
}

// Timestamp returns the event timestamp as a Go time.Time.
func (h *EventHeader) Timestamp() time.Time {
	// Convert nanoseconds since boot to absolute time
	// Note: This is a simplified conversion. In production, you'd want to
	// track the boot time and add it to the monotonic timestamp.
	return time.Unix(0, int64(h.TimestampNs))
}

// GetPID returns PID from header (for backward compatibility).
func (e *ExecEvent) GetPID() uint32 {
	return e.Hdr.PID
}

// GetCgroupID returns CgroupID from header (for backward compatibility).
func (e *ExecEvent) GetCgroupID() uint64 {
	return e.Hdr.CgroupID
}

// GetBlocked returns Blocked from header (for backward compatibility).
func (e *ExecEvent) GetBlocked() uint8 {
	return e.Hdr.Blocked
}

// GetPID returns PID from header.
func (e *FileOpenEvent) GetPID() uint32 {
	return e.Hdr.PID
}

// GetCgroupID returns CgroupID from header.
func (e *FileOpenEvent) GetCgroupID() uint64 {
	return e.Hdr.CgroupID
}

// GetBlocked returns Blocked from header.
func (e *FileOpenEvent) GetBlocked() uint8 {
	return e.Hdr.Blocked
}

// GetPID returns PID from header.
func (e *ConnectEvent) GetPID() uint32 {
	return e.Hdr.PID
}

// GetCgroupID returns CgroupID from header.
func (e *ConnectEvent) GetCgroupID() uint64 {
	return e.Hdr.CgroupID
}

// GetBlocked returns Blocked from header.
func (e *ConnectEvent) GetBlocked() uint8 {
	return e.Hdr.Blocked
}
