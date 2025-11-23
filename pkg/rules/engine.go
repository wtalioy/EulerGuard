package rules

import (
	"eulerguard/pkg/events"
	"strings"
)

type Engine struct {
	rules []Rule

	// Indexed lookups for exact matches - O(1)
	exactProcessNameRules map[string][]*Rule
	exactParentNameRules  map[string][]*Rule
	exactFilenameRules    map[string][]*Rule

	// Non-exact matches require iteration - O(m) where m is subset
	partialMatchRules []*Rule
	filepathRules     []*Rule
}

func NewEngine(rules []Rule) *Engine {
	e := &Engine{
		rules:                 rules,
		exactProcessNameRules: make(map[string][]*Rule),
		exactParentNameRules:  make(map[string][]*Rule),
		exactFilenameRules:    make(map[string][]*Rule),
		partialMatchRules:     make([]*Rule, 0),
		filepathRules:         make([]*Rule, 0),
	}

	// Build indexes
	e.buildIndexes()

	return e
}

func (e *Engine) buildIndexes() {
	for i := range e.rules {
		rule := &e.rules[i]

		// Set default match types for backward compatibility
		e.setDefaultMatchTypes(rule)

		// Index exec event rules
		if e.hasExecCriteria(rule) {
			e.indexExecRule(rule)
		}

		// Index file access rules
		if rule.Match.Filename != "" {
			e.exactFilenameRules[rule.Match.Filename] = append(
				e.exactFilenameRules[rule.Match.Filename], rule)
		}
		if rule.Match.FilePath != "" {
			e.filepathRules = append(e.filepathRules, rule)
		}
	}
}

// setDefaultMatchTypes sets default match types for backward compatibility
func (e *Engine) setDefaultMatchTypes(rule *Rule) {
	if rule.Match.ProcessName != "" && rule.Match.ProcessNameType == "" {
		rule.Match.ProcessNameType = MatchTypeContains
	}
	if rule.Match.ParentName != "" && rule.Match.ParentNameType == "" {
		rule.Match.ParentNameType = MatchTypeContains
	}
}

// hasExecCriteria checks if rule has exec event matching criteria
func (e *Engine) hasExecCriteria(rule *Rule) bool {
	return rule.Match.ProcessName != "" || rule.Match.ParentName != "" ||
		rule.Match.PID != 0 || rule.Match.PPID != 0
}

// indexExecRule indexes a rule for exec event matching
func (e *Engine) indexExecRule(rule *Rule) {
	indexed := false

	// Try to index by exact process name
	if rule.Match.ProcessName != "" && rule.Match.ProcessNameType == MatchTypeExact {
		e.exactProcessNameRules[rule.Match.ProcessName] = append(
			e.exactProcessNameRules[rule.Match.ProcessName], rule)
		indexed = true
	}

	// Try to index by exact parent name
	if rule.Match.ParentName != "" && rule.Match.ParentNameType == MatchTypeExact {
		e.exactParentNameRules[rule.Match.ParentName] = append(
			e.exactParentNameRules[rule.Match.ParentName], rule)
		indexed = true
	}

	// Add to partial match list if not fully indexed or has partial matches
	needsPartialMatch := !indexed ||
		rule.Match.ProcessNameType == MatchTypeContains ||
		rule.Match.ParentNameType == MatchTypeContains

	if needsPartialMatch {
		e.partialMatchRules = append(e.partialMatchRules, rule)
	}
}

// Match checks if an event matches any rules and returns alerts
func (e *Engine) Match(event events.ProcessedEvent) []Alert {
	var alerts []Alert
	checked := make(map[*Rule]bool) // Track checked rules to avoid duplicates

	// Strategy: Check indexed exact matches first (O(1)), then partial matches (O(m))

	// Helper to check and add matching rules
	checkRules := func(rulesToCheck []*Rule) {
		for _, rule := range rulesToCheck {
			if !checked[rule] && e.matchRule(*rule, event) {
				alerts = append(alerts, Alert{
					Rule:    *rule,
					Event:   event,
					Message: rule.Description,
				})
				checked[rule] = true
			}
		}
	}

	// 1. Check exact process name matches
	if rules, ok := e.exactProcessNameRules[event.Process]; ok {
		checkRules(rules)
	}

	// 2. Check exact parent name matches
	if rules, ok := e.exactParentNameRules[event.Parent]; ok {
		checkRules(rules)
	}

	// 3. Check partial match rules (contains, prefix, or complex conditions)
	checkRules(e.partialMatchRules)

	return alerts
}

// matchRule checks if a single rule matches the event
// Note: Strings are already normalized in ProcessedEvent, no need for TrimSpace
func (e *Engine) matchRule(rule Rule, event events.ProcessedEvent) bool {
	match := rule.Match

	// Check process name
	if match.ProcessName != "" && !matchString(event.Process, match.ProcessName, match.ProcessNameType) {
		return false
	}

	// Check parent name
	if match.ParentName != "" && !matchString(event.Parent, match.ParentName, match.ParentNameType) {
		return false
	}

	// Check PID
	if match.PID != 0 && event.Event.PID != match.PID {
		return false
	}

	// Check PPID
	if match.PPID != 0 && event.Event.PPID != match.PPID {
		return false
	}

	// Check container constraint
	if match.InContainer && event.Event.CgroupID == 1 {
		return false
	}

	return true
}

// matchString checks if a string matches based on the match type
func matchString(value, pattern string, matchType MatchType) bool {
	switch matchType {
	case MatchTypeExact:
		return value == pattern
	case MatchTypePrefix:
		return strings.HasPrefix(value, pattern)
	case MatchTypeContains:
		return strings.Contains(value, pattern)
	default:
		// Backward compatibility: default to contains
		return strings.Contains(value, pattern)
	}
}

func (e *Engine) GetRules() []Rule {
	return e.rules
}

func (e *Engine) RuleCount() int {
	return len(e.rules)
}

// MatchFile checks if a file access event matches any rules and returns the rule
func (e *Engine) MatchFile(filename string, pid uint32, cgroupID uint64) (bool, *Rule) {
	// Check exact filename matches (O(1) lookup)
	if rules, ok := e.exactFilenameRules[filename]; ok {
		for _, rule := range rules {
			if e.matchFileRule(*rule, filename, pid, cgroupID) {
				return true, rule
			}
		}
	}

	// Check prefix matches (O(m) where m = prefix rules)
	for _, rule := range e.filepathRules {
		if e.matchFileRule(*rule, filename, pid, cgroupID) {
			return true, rule
		}
	}

	return false, nil
}

// matchFileRule checks if a file access matches a rule
func (e *Engine) matchFileRule(rule Rule, filename string, pid uint32, cgroupID uint64) bool {
	match := rule.Match

	// Skip rules that don't have file matching criteria
	if match.Filename == "" && match.FilePath == "" {
		return false
	}

	// Check exact filename match
	if match.Filename != "" && filename != match.Filename {
		return false
	}

	// Check file path prefix match
	if match.FilePath != "" && !strings.HasPrefix(filename, match.FilePath) {
		return false
	}

	// Check container constraint
	if match.InContainer && cgroupID == 1 {
		return false
	}

	// Check PID constraint
	if match.PID != 0 && pid != match.PID {
		return false
	}

	return true
}
