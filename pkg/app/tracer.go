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
	"eulerguard/pkg/proc"

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

	kp, err := link.Kprobe("__x64_sys_execve", objs.HandleExecve, nil)
	if err != nil {
		return fmt.Errorf("attach kprobe: %w", err)
	}
	defer kp.Close()

	reader, err := ringbuf.NewReader(objs.Events)
	if err != nil {
		return fmt.Errorf("open ringbuf reader: %w", err)
	}
	defer reader.Close()

	go func() {
		<-ctx.Done()
		_ = reader.Close()
	}()

	resolver := proc.NewResolver(5 * time.Second)
	meter := metrics.NewRateMeter(2 * time.Second)
	printer := output.NewPrinter(t.opts.JSONLines, resolver, meter)

	log.Printf("EulerGuard execve tracer ready (BPF object: %s)", t.opts.BPFPath)

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

		ev, err := decodeExecEvent(record.RawSample)
		if err != nil {
			return err
		}

		printer.Print(ev)
	}
}

func decodeExecEvent(data []byte) (events.ExecEvent, error) {
	if len(data) < 8 {
		return events.ExecEvent{}, fmt.Errorf("exec event payload too small: %d bytes", len(data))
	}

	return events.ExecEvent{
		PID:  binary.LittleEndian.Uint32(data[0:4]),
		PPID: binary.LittleEndian.Uint32(data[4:8]),
	}, nil
}
