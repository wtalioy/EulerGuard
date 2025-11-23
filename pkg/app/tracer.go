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

	objs, err := ebpf.LoadExecveObjects(t.opts.BPFPath)
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

	printer, err := output.NewPrinter(t.opts.JSONLines, meter, t.opts.LogFile)
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

	// Create process tree
	processTree := proctree.NewProcessTree(30 * time.Minute)

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

		// Decode event type first
		if len(record.RawSample) < 1 {
			continue
		}

		eventType := record.RawSample[0]

		switch eventType {
		case 1: // EVENT_TYPE_EXEC
			ev, err := decodeExecEvent(record.RawSample)
			if err != nil {
				log.Printf("Error decoding exec event: %v", err)
				continue
			}

			// Add to process tree
			processTree.AddProcess(ev.PID, ev.PPID, ev.CgroupID, extractComm(ev.Comm))

			// Print the event and get the processed event
			processedEvent := printer.Print(ev)

			// Match against rules
			alerts := ruleEngine.Match(processedEvent)
			for _, alert := range alerts {
				printer.PrintAlert(alert)
			}

		case 2: // EVENT_TYPE_FILE_OPEN
			ev, err := decodeFileOpenEvent(record.RawSample)
			if err != nil {
				log.Printf("Error decoding file open event: %v", err)
				continue
			}

			// Check if file access matches any rules
			filename := extractFilename(ev.Filename)
			matched, rule := ruleEngine.MatchFile(filename, ev.PID, ev.CgroupID)
			if matched && rule != nil {
				// Get attack chain
				chain := processTree.GetAncestors(ev.PID)
				printer.PrintFileOpenAlert(ev, chain, rule)
			}
		}
	}
}

func decodeExecEvent(data []byte) (events.ExecEvent, error) {
	if len(data) < 49 {
		return events.ExecEvent{}, fmt.Errorf("exec event payload too small: %d bytes", len(data))
	}

	var ev events.ExecEvent
	// Skip type byte at index 0
	ev.PID = binary.LittleEndian.Uint32(data[1:5])
	ev.PPID = binary.LittleEndian.Uint32(data[5:9])
	ev.CgroupID = binary.LittleEndian.Uint64(data[9:17])
	copy(ev.Comm[:], data[17:33])
	copy(ev.PComm[:], data[33:49])

	return ev, nil
}

func decodeFileOpenEvent(data []byte) (*events.FileOpenEvent, error) {
	if len(data) < 273 { // 1 + 4 + 8 + 4 + 256 = 273
		return nil, fmt.Errorf("file open event too small: %d bytes", len(data))
	}

	ev := &events.FileOpenEvent{}
	// Skip type byte at index 0
	ev.PID = binary.LittleEndian.Uint32(data[1:5])
	ev.CgroupID = binary.LittleEndian.Uint64(data[5:13])
	ev.Flags = binary.LittleEndian.Uint32(data[13:17])
	copy(ev.Filename[:], data[17:273])

	return ev, nil
}

func extractComm(comm [16]byte) string {
	for i, b := range comm {
		if b == 0 {
			return string(comm[:i])
		}
	}
	return string(comm[:])
}

func extractFilename(filename [256]byte) string {
	for i, b := range filename {
		if b == 0 {
			return string(filename[:i])
		}
	}
	return string(filename[:])
}
