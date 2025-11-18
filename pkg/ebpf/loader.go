package ebpf

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/cilium/ebpf"
)

type ExecveObjects struct {
	HandleExecve *ebpf.Program `ebpf:"handle_execve"`
	Events       *ebpf.Map     `ebpf:"events"`
}

func LoadExecveObjects(objPath string) (*ExecveObjects, error) {
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

	objs := &ExecveObjects{}
	if err := spec.LoadAndAssign(objs, nil); err != nil {
		return nil, fmt.Errorf("load eBPF programs: %w", err)
	}

	return objs, nil
}

func (o *ExecveObjects) Close() error {
	if o == nil {
		return nil
	}

	var firstErr error
	if o.HandleExecve != nil {
		if err := o.HandleExecve.Close(); err != nil {
			firstErr = fmt.Errorf("close handle_execve: %w", err)
		}
	}
	if o.Events != nil {
		if err := o.Events.Close(); err != nil && firstErr == nil {
			firstErr = fmt.Errorf("close events map: %w", err)
		}
	}
	return firstErr
}
