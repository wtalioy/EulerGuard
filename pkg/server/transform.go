package server

import (
	"strconv"

	"aegis/pkg/events"
	"aegis/pkg/types"
	"aegis/pkg/utils"
)

// ExecToFrontend converts a backend ExecEvent to a frontend ExecEvent.
func ExecToFrontend(ev events.ExecEvent) types.ExecEvent {
	// Extract command line - prefer the full command_line field, fallback to filename, then comm
	commandLine := utils.ExtractCString(ev.CommandLine[:])
	if commandLine == "" {
		// Fallback to filename if command_line is empty
		commandLine = utils.ExtractCString(ev.Filename[:])
	}
	if commandLine == "" {
		// Final fallback to comm (process name)
		commandLine = utils.ExtractCString(ev.Hdr.Comm[:])
	}

	return types.ExecEvent{
		Type:        "exec",
		Timestamp:   ev.Hdr.Timestamp().UnixMilli(),
		PID:         ev.Hdr.PID,
		PPID:        ev.PPID,
		CgroupID:    strconv.FormatUint(ev.Hdr.CgroupID, 10),
		Comm:        utils.ExtractCString(ev.Hdr.Comm[:]),
		ParentComm:  utils.ExtractCString(ev.PComm[:]),
		CommandLine: commandLine,
		Blocked:     ev.Hdr.Blocked == 1,
	}
}

// FileToFrontend converts a backend FileOpenEvent to a frontend FileEvent.
func FileToFrontend(ev events.FileOpenEvent, filename string) types.FileEvent {
	return types.FileEvent{
		Type:      "file",
		Timestamp: ev.Hdr.Timestamp().UnixMilli(),
		PID:       ev.Hdr.PID,
		CgroupID:  strconv.FormatUint(ev.Hdr.CgroupID, 10),
		Flags:     ev.Flags,
		Ino:       ev.Ino,
		Dev:       ev.Dev,
		Filename:  filename,
		Blocked:   ev.Hdr.Blocked == 1,
	}
}

// ConnectToFrontend converts a backend ConnectEvent to a frontend ConnectEvent.
func ConnectToFrontend(ev events.ConnectEvent, addr string, processName string) types.ConnectEvent {
	return types.ConnectEvent{
		Type:        "connect",
		Timestamp:   ev.Hdr.Timestamp().UnixMilli(),
		PID:         ev.Hdr.PID,
		ProcessName: processName,
		CgroupID:    strconv.FormatUint(ev.Hdr.CgroupID, 10),
		Family:      ev.Family,
		Port:        ev.Port,
		Addr:        addr,
		Blocked:     ev.Hdr.Blocked == 1,
	}
}
