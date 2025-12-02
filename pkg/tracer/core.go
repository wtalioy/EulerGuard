package tracer

import (
	"errors"
	"fmt"
	"log"
	"strings"
	"syscall"

	"eulerguard/pkg/config"
	"eulerguard/pkg/ebpf"
	"eulerguard/pkg/events"
	"eulerguard/pkg/proc"
	"eulerguard/pkg/rules"
	"eulerguard/pkg/types"
	"eulerguard/pkg/utils"
	"eulerguard/pkg/workload"

	cebpf "github.com/cilium/ebpf"
	"github.com/cilium/ebpf/link"
	"github.com/cilium/ebpf/ringbuf"
)

type Core struct {
	Objs             *ebpf.LSMObjects
	Links            []link.Link
	Reader           *ringbuf.Reader
	Rules            []types.Rule
	RuleEngine       *rules.Engine
	ProcessTree      *proc.ProcessTree
	WorkloadRegistry *workload.Registry
	RulesPath        string
}

func Init(opts config.Options) (*Core, error) {
	c := &Core{}

	c.ProcessTree = proc.NewProcessTree(
		opts.ProcessTreeMaxAge,
		opts.ProcessTreeMaxSize,
		opts.ProcessTreeMaxChainLength,
	)

	c.WorkloadRegistry = workload.NewRegistry(1000)

	objs, err := ebpf.LoadLSMObjects(opts.BPFPath, opts.RingBufferSize)
	if err != nil {
		return nil, fmt.Errorf("load eBPF LSM objects: %w", err)
	}
	c.Objs = objs
	if c.ProcessTree != nil && c.Objs.PidToPpid != nil {
		c.ProcessTree.SetPIDResolver(newPIDResolver(c.Objs.PidToPpid))
	}

	links, err := AttachLSMHooks(objs)
	if err != nil {
		objs.Close()
		return nil, fmt.Errorf("attach LSM hooks: %w", err)
	}
	c.Links = links

	reader, err := ringbuf.NewReader(objs.Events)
	if err != nil {
		CloseLinks(links)
		objs.Close()
		return nil, fmt.Errorf("create ringbuf reader: %w", err)
	}
	c.Reader = reader

	c.RulesPath = opts.RulesPath
	c.Rules, c.RuleEngine = LoadRules(opts.RulesPath)

	if err := PopulateMonitoredFiles(objs.MonitoredFiles, c.Rules, opts.RulesPath); err != nil {
		log.Printf("Warning: failed to populate monitored files: %v", err)
	}
	if err := PopulateBlockedPorts(objs.BlockedPorts, c.Rules); err != nil {
		log.Printf("Warning: failed to populate blocked ports: %v", err)
	}

	return c, nil
}

func (c *Core) ReloadRules() error {
	newRules, newEngine := LoadRules(c.RulesPath)
	c.Rules = newRules
	c.RuleEngine = newEngine

	if c.Objs != nil {
		if c.Objs.MonitoredFiles != nil {
			if err := RepopulateMonitoredFiles(c.Objs.MonitoredFiles, c.Rules, c.RulesPath); err != nil {
				return fmt.Errorf("failed to repopulate monitored files: %w", err)
			}
		}
		if c.Objs.BlockedPorts != nil {
			if err := RepopulateBlockedPorts(c.Objs.BlockedPorts, c.Rules); err != nil {
				return fmt.Errorf("failed to repopulate blocked ports: %w", err)
			}
		}
	}

	log.Printf("Rules reloaded: %d rules from %s", len(c.Rules), c.RulesPath)
	return nil
}

func RepopulateMonitoredFiles(bpfMap *cebpf.Map, ruleList []types.Rule, rulesPath string) error {
	if bpfMap == nil {
		return fmt.Errorf("monitored_files map is nil")
	}

	var key [events.PathMaxLen]byte
	var val uint8
	iter := bpfMap.Iterate()
	keysToDelete := make([][]byte, 0)
	for iter.Next(&key, &val) {
		keyCopy := make([]byte, events.PathMaxLen)
		copy(keyCopy, key[:])
		keysToDelete = append(keysToDelete, keyCopy)
	}
	for _, k := range keysToDelete {
		_ = bpfMap.Delete(k)
	}

	return PopulateMonitoredFiles(bpfMap, ruleList, rulesPath)
}

func RepopulateBlockedPorts(bpfMap *cebpf.Map, ruleList []types.Rule) error {
	if bpfMap == nil {
		return fmt.Errorf("blocked_ports map is nil")
	}

	var key uint16
	var val uint8
	iter := bpfMap.Iterate()
	keysToDelete := make([]uint16, 0)
	for iter.Next(&key, &val) {
		keysToDelete = append(keysToDelete, key)
	}
	for _, k := range keysToDelete {
		_ = bpfMap.Delete(k)
	}

	return PopulateBlockedPorts(bpfMap, ruleList)
}

func (c *Core) Close() {
	if c.Reader != nil {
		c.Reader.Close()
	}
	CloseLinks(c.Links)
	if c.Objs != nil {
		c.Objs.Close()
	}
}

func AttachLSMHooks(objs *ebpf.LSMObjects) ([]link.Link, error) {
	var links []link.Link

	lsmBprm, err := link.AttachLSM(link.LSMOptions{
		Program: objs.LsmBprmCheck,
	})
	if err != nil {
		return nil, fmt.Errorf("attach bprm_check_security LSM: %w", err)
	}
	links = append(links, lsmBprm)

	lsmFileOpen, err := link.AttachLSM(link.LSMOptions{
		Program: objs.LsmFileOpen,
	})
	if err != nil {
		CloseLinks(links)
		return nil, fmt.Errorf("attach file_open LSM: %w", err)
	}
	links = append(links, lsmFileOpen)

	lsmSocketConnect, err := link.AttachLSM(link.LSMOptions{
		Program: objs.LsmSocketConnect,
	})
	if err != nil {
		CloseLinks(links)
		return nil, fmt.Errorf("attach socket_connect LSM: %w", err)
	}
	links = append(links, lsmSocketConnect)

	log.Printf("Attached 3 BPF LSM hooks for active defense")
	return links, nil
}

func CloseLinks(links []link.Link) {
	for _, l := range links {
		_ = l.Close()
	}
}

func LoadRules(rulesPath string) ([]types.Rule, *rules.Engine) {
	loadedRules, err := rules.LoadRules(rulesPath)
	if err != nil {
		log.Printf("Warning: failed to load rules from %s: %v", rulesPath, err)
		loadedRules = []types.Rule{}
	} else {
		log.Printf("Loaded %d detection rules from %s", len(loadedRules), rulesPath)
	}
	return loadedRules, rules.NewEngine(loadedRules)
}

func PopulateMonitoredFiles(bpfMap *cebpf.Map, ruleList []types.Rule, rulesPath string) error {
	if bpfMap == nil {
		return fmt.Errorf("monitored_files map is nil")
	}

	fileActions := make(map[string]uint8)
	for _, rule := range ruleList {
		paths := rule.Match.ExactPathKeys()
		if len(paths) == 0 {
			continue
		}

		for _, path := range paths {
			key := extractParentFilename(path)
			if key == "" {
				continue
			}

			var action uint8
			if rule.Action == types.ActionBlock {
				action = types.BPFActionBlock
			} else {
				action = types.BPFActionMonitor
			}

			if existing, ok := fileActions[key]; !ok || action > existing {
				fileActions[key] = action
			}
		}
	}

	if len(fileActions) == 0 {
		log.Printf("Warning: No file access rules found in %s", rulesPath)
		return nil
	}

	countMonitor := 0
	countBlock := 0
	for filename, action := range fileActions {
		key := make([]byte, events.PathMaxLen)
		copy(key, []byte(filename))
		if err := bpfMap.Put(key, action); err != nil {
			return fmt.Errorf("add file %q to BPF map: %w", filename, err)
		}
		if action == types.BPFActionBlock {
			countBlock++
		} else {
			countMonitor++
		}
	}

	log.Printf("Populated BPF map with %d monitored files (%d block, %d monitor)",
		len(fileActions), countBlock, countMonitor)
	return nil
}

func extractParentFilename(path string) string {
	for len(path) > 0 && path[0] == '/' {
		path = path[1:]
	}
	if path == "" {
		return ""
	}

	segments := strings.FieldsFunc(path, func(r rune) bool { return r == '/' })
	if len(segments) == 0 {
		return ""
	}

	start := max(len(segments)-3, 0)
	return strings.Join(segments[start:], "/")
}

func PopulateBlockedPorts(bpfMap *cebpf.Map, ruleList []types.Rule) error {
	if bpfMap == nil {
		return fmt.Errorf("blocked_ports map is nil")
	}

	portActions := make(map[uint16]uint8)
	for _, rule := range ruleList {
		if rule.Match.DestPort == 0 {
			continue
		}

		var action uint8
		if rule.Action == types.ActionBlock {
			action = types.BPFActionBlock
		} else {
			action = types.BPFActionMonitor
		}

		port := rule.Match.DestPort
		if existing, ok := portActions[port]; !ok || action > existing {
			portActions[port] = action
		}
	}

	if len(portActions) == 0 {
		return nil
	}

	countMonitor := 0
	countBlock := 0
	for port, action := range portActions {
		if err := bpfMap.Put(port, action); err != nil {
			return fmt.Errorf("add port %d to BPF map: %w", port, err)
		}
		if action == types.BPFActionBlock {
			countBlock++
		} else {
			countMonitor++
		}
	}

	log.Printf("Populated BPF map with %d monitored ports (%d block, %d monitor)",
		len(portActions), countBlock, countMonitor)
	return nil
}

func newPIDResolver(m *cebpf.Map) proc.PIDResolver {
	if m == nil {
		return nil
	}

	return func(pid uint32) (uint32, bool) {
		var parent uint32
		if err := m.Lookup(&pid, &parent); err != nil {
			return 0, false
		}
		return parent, true
	}
}

func EventLoop(reader *ringbuf.Reader, handlers *events.HandlerChain, processTree *proc.ProcessTree, registry *workload.Registry) error {
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

		if len(record.RawSample) < 1 {
			continue
		}

		DispatchEvent(record.RawSample, handlers, processTree, registry)
	}
}

func DispatchEvent(data []byte, handlers *events.HandlerChain, processTree *proc.ProcessTree, registry *workload.Registry) {
	switch events.EventType(data[0]) {
	case events.EventTypeExec:
		ev, err := events.DecodeExecEvent(data)
		if err != nil {
			log.Printf("Error decoding exec event: %v", err)
			return
		}
		processTree.AddProcess(ev.PID, ev.PPID, ev.CgroupID, utils.ExtractCString(ev.Comm[:]))
		if registry != nil {
			cgroupPath := proc.ResolveCgroupPath(ev.PID, ev.CgroupID)
			registry.RecordExec(ev.CgroupID, cgroupPath)
		}
		handlers.HandleExec(ev)

	case events.EventTypeFileOpen:
		ev, err := events.DecodeFileOpenEvent(data)
		if err != nil {
			log.Printf("Error decoding file open event: %v", err)
			return
		}
		if registry != nil {
			cgroupPath := proc.ResolveCgroupPath(ev.PID, ev.CgroupID)
			registry.RecordFile(ev.CgroupID, cgroupPath)
		}
		filename := utils.ExtractCString(ev.Filename[:])
		handlers.HandleFileOpen(ev, filename)

	case events.EventTypeConnect:
		ev, err := events.DecodeConnectEvent(data)
		if err != nil {
			log.Printf("Error decoding connect event: %v", err)
			return
		}
		if registry != nil {
			cgroupPath := proc.ResolveCgroupPath(ev.PID, ev.CgroupID)
			registry.RecordConnect(ev.CgroupID, cgroupPath)
		}
		handlers.HandleConnect(ev)
	}
}
