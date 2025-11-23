package proctree

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

type ProcessInfo struct {
	PID       uint32
	PPID      uint32
	CgroupID  uint64
	Comm      string
	Timestamp time.Time
}

type ProcessTree struct {
	processes sync.Map
	maxAge    time.Duration
}

func NewProcessTree(maxAge time.Duration) *ProcessTree {
	pt := &ProcessTree{
		maxAge: maxAge,
	}

	// Seed from /proc on startup
	if err := pt.seedFromProc(); err != nil {
		log.Printf("Warning: failed to seed process tree from /proc: %v", err)
	}

	// Start cleanup goroutine
	go pt.cleanupLoop()

	return pt
}

func (pt *ProcessTree) seedFromProc() error {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return err
	}

	count := 0
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		pid, err := strconv.ParseUint(entry.Name(), 10, 32)
		if err != nil {
			continue
		}

		info, err := pt.readProcInfo(uint32(pid))
		if err != nil {
			continue
		}

		pt.processes.Store(uint32(pid), info)
		count++
	}

	log.Printf("Process tree seeded with %d processes from /proc", count)
	return nil
}

func (pt *ProcessTree) readProcInfo(pid uint32) (*ProcessInfo, error) {
	// Read /proc/[pid]/stat for PPID and comm
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
	if err != nil {
		return nil, err
	}

	// Parse stat file: pid (comm) state ppid ...
	// Need to handle comm with spaces and parentheses
	str := string(data)
	
	// Find the last ')' to handle comm with spaces
	commEnd := strings.LastIndex(str, ")")
	if commEnd == -1 {
		return nil, fmt.Errorf("invalid stat format")
	}
	
	commStart := strings.Index(str, "(")
	if commStart == -1 {
		return nil, fmt.Errorf("invalid stat format")
	}
	
	comm := str[commStart+1 : commEnd]
	
	// Parse fields after comm
	fields := strings.Fields(str[commEnd+1:])
	if len(fields) < 2 {
		return nil, fmt.Errorf("invalid stat format")
	}
	
	ppid, err := strconv.ParseUint(fields[1], 10, 32)
	if err != nil {
		return nil, err
	}

	// Read cgroup ID
	cgroupID := pt.readCgroupID(pid)

	return &ProcessInfo{
		PID:       pid,
		PPID:      uint32(ppid),
		CgroupID:  cgroupID,
		Comm:      comm,
		Timestamp: time.Now(),
	}, nil
}

func (pt *ProcessTree) readCgroupID(pid uint32) uint64 {
	// Try to read cgroup info from /proc/[pid]/cgroup
	// For now, simplified: return 1 for host processes
	// In production, parse /proc/[pid]/cgroup to get actual cgroup ID
	return 1
}

func (pt *ProcessTree) AddProcess(pid, ppid uint32, cgroupID uint64, comm string) {
	info := &ProcessInfo{
		PID:       pid,
		PPID:      ppid,
		CgroupID:  cgroupID,
		Comm:      comm,
		Timestamp: time.Now(),
	}
	pt.processes.Store(pid, info)
}

func (pt *ProcessTree) GetProcess(pid uint32) (*ProcessInfo, bool) {
	val, ok := pt.processes.Load(pid)
	if !ok {
		return nil, false
	}
	return val.(*ProcessInfo), true
}

func (pt *ProcessTree) GetAncestors(pid uint32) []*ProcessInfo {
	var chain []*ProcessInfo
	visited := make(map[uint32]bool)

	currentPID := pid
	for currentPID != 0 && currentPID != 1 {
		// Prevent infinite loops
		if visited[currentPID] {
			break
		}
		visited[currentPID] = true

		info, ok := pt.GetProcess(currentPID)
		if !ok {
			break
		}

		chain = append(chain, info)
		currentPID = info.PPID

		// Stop at container boundary (cgroup change)
		if len(chain) > 1 && info.CgroupID != chain[0].CgroupID {
			break
		}

		// Safety limit
		if len(chain) > 50 {
			break
		}
	}

	return chain
}

func (pt *ProcessTree) cleanupLoop() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		pt.cleanup()
	}
}

func (pt *ProcessTree) cleanup() {
	now := time.Now()
	count := 0
	pt.processes.Range(func(key, value interface{}) bool {
		info := value.(*ProcessInfo)
		if now.Sub(info.Timestamp) > pt.maxAge {
			pt.processes.Delete(key)
			count++
		}
		return true
	})
	
	if count > 0 {
		log.Printf("Cleaned up %d old processes from tree", count)
	}
}

