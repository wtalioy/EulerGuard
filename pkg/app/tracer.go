package app

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"log"
	"os"
	"syscall"
	"time"

	"eulerguard/pkg/config"
	"eulerguard/pkg/ebpf"
	"eulerguard/pkg/events"
	"eulerguard/pkg/metrics"
	"eulerguard/pkg/output"
	"eulerguard/pkg/proctree"
	"eulerguard/pkg/rules"
	"eulerguard/pkg/utils"

	cebpf "github.com/cilium/ebpf"
	"github.com/cilium/ebpf/link"
	"github.com/cilium/ebpf/ringbuf"
)

type ExecveTracer struct {
	opts config.Options
}

func NewExecveTracer(opts config.Options) *ExecveTracer {
	return &ExecveTracer{opts: opts}
}

func (t *ExecveTracer) Run(ctx context.Context) error {
	if os.Geteuid() != 0 {
		return fmt.Errorf("must run as root (current euid=%d)", os.Geteuid())
	}

	objs, err := ebpf.LoadExecveObjects(t.opts.BPFPath, t.opts.RingBufferSize)
	if err != nil {
		return err
	}
	defer objs.Close()

	tp, err := link.Tracepoint("sched", "sched_process_exec", objs.HandleExec, nil)
	if err != nil {
		return fmt.Errorf("attach tracepoint: %w", err)
	}
	defer tp.Close()

	// Attach tracepoint for openat
	tpOpenat, err := link.Tracepoint("syscalls", "sys_enter_openat", objs.TracepointOpenat, nil)
	if err != nil {
		return fmt.Errorf("attach tracepoint openat: %w", err)
	}
	defer tpOpenat.Close()

	reader, err := ringbuf.NewReader(objs.Events)
	if err != nil {
		return fmt.Errorf("open ringbuf reader: %w", err)
	}
	defer reader.Close()

	go func() {
		<-ctx.Done()
		_ = reader.Close()
	}()

	meter := metrics.NewRateMeter(2 * time.Second)

	printer, err := output.NewPrinter(t.opts.JSONLines, meter, t.opts.LogFile, t.opts.LogBufferSize)
	if err != nil {
		return fmt.Errorf("failed to create printer: %w", err)
	}
	defer printer.Close()

	// Load rules
	loadedRules, err := rules.LoadRules(t.opts.RulesPath)
	if err != nil {
		log.Printf("Warning: failed to load rules from %s: %v", t.opts.RulesPath, err)
		log.Printf("Continuing without rules...")
		loadedRules = []rules.Rule{}
	} else {
		log.Printf("Loaded %d detection rules from %s", len(loadedRules), t.opts.RulesPath)
	}

	ruleEngine := rules.NewEngine(loadedRules)

	// Populate monitored_paths BPF map from rules
	if err := t.populateMonitoredPaths(objs.MonitoredPaths, loadedRules); err != nil {
		log.Printf("Warning: failed to populate monitored paths: %v", err)
	}

	// Create process tree
	processTree := proctree.NewProcessTree(t.opts.ProcessTreeMaxAge, t.opts.ProcessTreeMaxSize)

	log.Printf("EulerGuard tracer ready (BPF object: %s)", t.opts.BPFPath)

	for {
		record, err := reader.Read()
		if errors.Is(err, ringbuf.ErrClosed) {
			return nil
		}
		if err != nil {
			if errors.Is(err, syscall.EINTR) {
				continue
			}
			return fmt.Errorf("read ringbuf: %w", err)
		}

		// Process event based on type
		if len(record.RawSample) < 1 {
			continue
		}

		switch events.EventType(record.RawSample[0]) {
		case events.EventTypeExec:
			t.handleExecEvent(record.RawSample, processTree, printer, ruleEngine)
		case events.EventTypeFileOpen:
			t.handleFileOpenEvent(record.RawSample, processTree, printer, ruleEngine)
		}
	}
}

// handleExecEvent processes exec events
func (t *ExecveTracer) handleExecEvent(data []byte, processTree *proctree.ProcessTree,
	printer *output.Printer, ruleEngine *rules.Engine) {

	ev, err := decodeExecEvent(data)
	if err != nil {
		log.Printf("Error decoding exec event: %v", err)
		return
	}

	// Add to process tree
	processTree.AddProcess(ev.PID, ev.PPID, ev.CgroupID, utils.ExtractCString(ev.Comm[:]))

	// Print the event and get the processed event
	processedEvent := printer.Print(ev)

	// Match against rules and print alerts
	for _, alert := range ruleEngine.Match(processedEvent) {
		printer.PrintAlert(alert)
	}
}

// handleFileOpenEvent processes file open events
func (t *ExecveTracer) handleFileOpenEvent(data []byte, processTree *proctree.ProcessTree,
	printer *output.Printer, ruleEngine *rules.Engine) {

	ev, err := decodeFileOpenEvent(data)
	if err != nil {
		log.Printf("Error decoding file open event: %v", err)
		return
	}

	// Check if file access matches any rules
	filename := utils.ExtractCString(ev.Filename[:])
	if matched, rule := ruleEngine.MatchFile(filename, ev.PID, ev.CgroupID); matched && rule != nil {
		chain := processTree.GetAncestors(ev.PID)
		printer.PrintFileOpenAlert(&ev, chain, rule, filename)
	}
}

const (
	// Event sizes: type(1) + fields
	minExecEventSize     = 1 + 4 + 4 + 8 + events.TaskCommLen + events.TaskCommLen // 49 bytes
	minFileOpenEventSize = 1 + 4 + 8 + 4 + events.PathMaxLen                       // 273 bytes
)

func decodeExecEvent(data []byte) (events.ExecEvent, error) {
	if len(data) < minExecEventSize {
		return events.ExecEvent{}, fmt.Errorf("exec event payload too small: %d bytes", len(data))
	}

	var ev events.ExecEvent
	offset := 1 // Skip type byte at index 0
	ev.PID = binary.LittleEndian.Uint32(data[offset : offset+4])
	offset += 4
	ev.PPID = binary.LittleEndian.Uint32(data[offset : offset+4])
	offset += 4
	ev.CgroupID = binary.LittleEndian.Uint64(data[offset : offset+8])
	offset += 8
	copy(ev.Comm[:], data[offset:offset+16])
	offset += 16
	copy(ev.PComm[:], data[offset:offset+16])

	return ev, nil
}

func decodeFileOpenEvent(data []byte) (events.FileOpenEvent, error) {
	if len(data) < minFileOpenEventSize {
		return events.FileOpenEvent{}, fmt.Errorf("file open event too small: %d bytes", len(data))
	}

	var ev events.FileOpenEvent
	offset := 1 // Skip type byte at index 0
	ev.PID = binary.LittleEndian.Uint32(data[offset : offset+4])
	offset += 4
	ev.CgroupID = binary.LittleEndian.Uint64(data[offset : offset+8])
	offset += 8
	ev.Flags = binary.LittleEndian.Uint32(data[offset : offset+4])
	offset += 4
	copy(ev.Filename[:], data[offset:offset+events.PathMaxLen])

	return ev, nil
}

// populateMonitoredPaths extracts file paths from rules and populates the BPF map
func (t *ExecveTracer) populateMonitoredPaths(monitoredPathsMap *cebpf.Map, loadedRules []rules.Rule) error {
	// Extract unique paths from rules
	pathSet := extractMonitoredPaths(loadedRules)

	// Populate BPF map with paths
	return populateBPFMap(monitoredPathsMap, pathSet)
}

// extractMonitoredPaths gets unique file paths from rules
func extractMonitoredPaths(rules []rules.Rule) map[string]bool {
	pathSet := make(map[string]bool)

	for _, rule := range rules {
		if rule.Match.Filename != "" {
			pathSet[rule.Match.Filename] = true
		}
		if rule.Match.FilePath != "" {
			pathSet[rule.Match.FilePath] = true
		}
	}

	// If no paths specified, add common sensitive paths as defaults
	if len(pathSet) == 0 {
		for _, path := range []string{"/etc/", "/root/", "/home/"} {
			pathSet[path] = true
		}
	}

	return pathSet
}

// populateBPFMap adds paths to the BPF monitored_paths map
func populateBPFMap(monitoredPathsMap *cebpf.Map, paths map[string]bool) error {
	count := 0
	enabled := uint8(1)

	for path := range paths {
		var keyBuf [events.PathMaxLen]byte
		copy(keyBuf[:], path)

		if err := monitoredPathsMap.Put(keyBuf, enabled); err != nil {
			log.Printf("Warning: failed to add path '%s' to BPF map: %v", path, err)
			continue
		}
		count++
	}

	log.Printf("Populated %d monitored paths in BPF map", count)
	return nil
}
