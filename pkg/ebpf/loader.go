package ebpf

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/cilium/ebpf"
)

type LSMObjects struct {
	LsmBprmCheck     *ebpf.Program `ebpf:"lsm_bprm_check"`
	LsmFileOpen      *ebpf.Program `ebpf:"lsm_file_open"`
	LsmSocketConnect *ebpf.Program `ebpf:"lsm_socket_connect"`

	Events         *ebpf.Map `ebpf:"events"`
	MonitoredFiles *ebpf.Map `ebpf:"monitored_files"`
	BlockedPorts   *ebpf.Map `ebpf:"blocked_ports"`
}

func LoadLSMObjects(objPath string, ringBufSize int) (*LSMObjects, error) {
	absPath, err := filepath.Abs(objPath)
	if err != nil {
		return nil, fmt.Errorf("resolve bpf path: %w", err)
	}
	if _, err := os.Stat(absPath); err != nil {
		return nil, fmt.Errorf("stat bpf object: %w", err)
	}

	spec, err := ebpf.LoadCollectionSpec(absPath)
	if err != nil {
		return nil, fmt.Errorf("load collection spec: %w", err)
	}

	if ringBufSize > 0 {
		if eventsSpec, ok := spec.Maps["events"]; ok {
			eventsSpec.MaxEntries = uint32(ringBufSize)
		}
	}

	objs := &LSMObjects{}
	if err := spec.LoadAndAssign(objs, nil); err != nil {
		return nil, fmt.Errorf("load eBPF LSM programs: %w", err)
	}

	return objs, nil
}

func (o *LSMObjects) Close() error {
	if o == nil {
		return nil
	}

	var firstErr error

	if o.LsmBprmCheck != nil {
		if err := o.LsmBprmCheck.Close(); err != nil {
			firstErr = fmt.Errorf("close lsm_bprm_check: %w", err)
		}
	}
	if o.LsmFileOpen != nil {
		if err := o.LsmFileOpen.Close(); err != nil && firstErr == nil {
			firstErr = fmt.Errorf("close lsm_file_open: %w", err)
		}
	}
	if o.LsmSocketConnect != nil {
		if err := o.LsmSocketConnect.Close(); err != nil && firstErr == nil {
			firstErr = fmt.Errorf("close lsm_socket_connect: %w", err)
		}
	}

	if o.Events != nil {
		if err := o.Events.Close(); err != nil && firstErr == nil {
			firstErr = fmt.Errorf("close events map: %w", err)
		}
	}
	if o.MonitoredFiles != nil {
		if err := o.MonitoredFiles.Close(); err != nil && firstErr == nil {
			firstErr = fmt.Errorf("close monitored_files map: %w", err)
		}
	}
	if o.BlockedPorts != nil {
		if err := o.BlockedPorts.Close(); err != nil && firstErr == nil {
			firstErr = fmt.Errorf("close blocked_ports map: %w", err)
		}
	}

	return firstErr
}
