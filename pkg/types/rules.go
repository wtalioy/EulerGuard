package types

import (
	"log"
	"net"
	"os"
	"strings"
	"syscall"

	"eulerguard/pkg/events"
	"eulerguard/pkg/utils"
)

type MatchType string

type ActionType string

const (
	ActionAllow ActionType = "allow"
	ActionAlert ActionType = "alert"
	ActionBlock ActionType = "block"
)

const (
	BPFActionMonitor uint8 = 1
	BPFActionBlock   uint8 = 2
)

const (
	MatchTypeExact    MatchType = "exact"
	MatchTypeContains MatchType = "contains"
	MatchTypePrefix   MatchType = "prefix"
)

type RuleType string

const (
	RuleTypeExec    RuleType = "exec"
	RuleTypeFile    RuleType = "file"
	RuleTypeConnect RuleType = "connect"
)

type InodeKey struct {
	Ino uint64
	Dev uint64
}

type Rule struct {
	Name        string         `yaml:"name"`
	Description string         `yaml:"description"`
	Severity    string         `yaml:"severity"`
	Match       MatchCondition `yaml:"match"`
	Action      ActionType     `yaml:"action"`
	Type        RuleType       `yaml:"type,omitempty"`
}

func (r *Rule) DeriveType() RuleType {
	if r.Type != "" {
		return r.Type
	}
	if len(r.Match.ExactPathKeys()) > 0 || len(r.Match.PrefixPathKeys()) > 0 {
		return RuleTypeFile
	}
	if r.Match.DestPort != 0 || r.Match.DestIP != "" {
		return RuleTypeConnect
	}
	return RuleTypeExec
}

type MatchCondition struct {
	ProcessName     string     `yaml:"process_name,omitempty"`
	ProcessNameType MatchType  `yaml:"process_name_type,omitempty"`
	ParentName      string     `yaml:"parent_name,omitempty"`
	ParentNameType  MatchType  `yaml:"parent_name_type,omitempty"`
	PID             uint32     `yaml:"pid,omitempty"`
	PPID            uint32     `yaml:"ppid,omitempty"`
	CgroupID        string     `yaml:"cgroup_id,omitempty"`
	Filename        string     `yaml:"filename,omitempty"`
	DestPort        uint16     `yaml:"dest_port,omitempty"`
	DestIP          string     `yaml:"dest_ip,omitempty"`
	destIPNet       *net.IPNet `yaml:"-"`
	destIPPrepared  bool       `yaml:"-"`
	inode           InodeKey   `yaml:"-"`
	inodeResolved   bool       `yaml:"-"`
	pathExactKeys   []string   `yaml:"-"`
	pathPrefixKeys  []string   `yaml:"-"`
}

type RuleSet struct {
	Rules []Rule `yaml:"rules"`
}

type MatchedAlert struct {
	Rule    Rule
	Event   events.ProcessedEvent
	Message string
}

func (m *MatchCondition) Prepare() {
	if m == nil {
		return
	}

	if m.Filename != "" {
		m.prepareFilenameKeys(m.Filename)
		m.prepareInode()
	} else {
		m.pathExactKeys = nil
		m.pathPrefixKeys = nil
	}

	if !m.destIPPrepared {
		m.destIPPrepared = true
		if m.DestIP == "" {
			m.destIPNet = nil
			return
		}
		if _, cidr, err := net.ParseCIDR(m.DestIP); err == nil {
			m.destIPNet = cidr
		} else {
			m.destIPNet = nil
		}
	}
}

func (m *MatchCondition) MatchIP(eventIP string) bool {
	if m == nil || m.DestIP == "" {
		return true
	}
	if m.destIPNet != nil {
		ip := net.ParseIP(eventIP)
		return ip != nil && m.destIPNet.Contains(ip)
	}
	return eventIP == m.DestIP
}

func (m *MatchCondition) InodeKey() (InodeKey, bool) {
	if m == nil || !m.inodeResolved {
		return InodeKey{}, false
	}
	return m.inode, true
}

func (m *MatchCondition) ExactPathKeys() []string {
	if m == nil {
		return nil
	}
	return m.pathExactKeys
}

func (m *MatchCondition) PrefixPathKeys() []string {
	if m == nil {
		return nil
	}
	return m.pathPrefixKeys
}

func (m *MatchCondition) prepareInode() {
	if m.inodeResolved {
		return
	}

	path := m.Filename
	if path == "" {
		return
	}

	info, err := os.Stat(path)
	if err != nil {
		log.Printf("Skipping file rule for %s: %v", path, err)
		return
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		log.Printf("Skipping file rule for %s: unsupported stat type", path)
		return
	}

	m.inode = InodeKey{
		Ino: stat.Ino,
		Dev: uint64(stat.Dev),
	}
	m.inodeResolved = true
}

func (m *MatchCondition) prepareFilenameKeys(raw string) {
	path := strings.TrimSpace(raw)
	if path == "" {
		m.pathExactKeys = nil
		m.pathPrefixKeys = nil
		return
	}

	if strings.HasSuffix(path, "*") {
		base := strings.TrimSuffix(path, "*")
		base = strings.TrimSuffix(base, "/")
		variants := utils.PathVariants(base)
		m.pathExactKeys = nil
		m.pathPrefixKeys = normalizePrefixVariants(variants)
		return
	}

	variants := utils.PathVariants(path)
	m.pathExactKeys = variants
	m.pathPrefixKeys = nil
}

func normalizePrefixVariants(variants []string) []string {
	if len(variants) == 0 {
		return nil
	}
	out := make([]string, 0, len(variants))
	for _, v := range variants {
		if v == "" {
			continue
		}
		prefix := v
		if prefix != "/" && !strings.HasSuffix(prefix, "/") {
			prefix += "/"
		}
		out = append(out, prefix)
	}
	return out
}
