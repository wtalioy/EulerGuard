package rules

import (
	"eulerguard/pkg/events"
	"eulerguard/pkg/types"
)

type execMatcher struct {
	exactProcessNameRules map[string][]*types.Rule
	exactParentNameRules  map[string][]*types.Rule
	partialMatchRules     []*types.Rule
}

func newExecMatcher(rules []types.Rule) *execMatcher {
	matcher := &execMatcher{
		exactProcessNameRules: make(map[string][]*types.Rule),
		exactParentNameRules:  make(map[string][]*types.Rule),
		partialMatchRules:     make([]*types.Rule, 0),
	}
	for i := range rules {
		rule := &rules[i]
		setDefaultMatchTypes(rule)
		if hasExecCriteria(rule) {
			matcher.indexRule(rule)
		}
	}
	return matcher
}

func setDefaultMatchTypes(rule *types.Rule) {
	if rule.Match.ProcessName != "" && rule.Match.ProcessNameType == "" {
		rule.Match.ProcessNameType = types.MatchTypeContains
	}
	if rule.Match.ParentName != "" && rule.Match.ParentNameType == "" {
		rule.Match.ParentNameType = types.MatchTypeContains
	}
}

func hasExecCriteria(rule *types.Rule) bool {
	m := rule.Match
	return m.ProcessName != "" || m.ParentName != "" || m.PID != 0 || m.PPID != 0
}

func (m *execMatcher) indexRule(rule *types.Rule) {
	indexed := false
	if rule.Match.ProcessName != "" && rule.Match.ProcessNameType == types.MatchTypeExact {
		m.exactProcessNameRules[rule.Match.ProcessName] = append(
			m.exactProcessNameRules[rule.Match.ProcessName], rule)
		indexed = true
	}
	if rule.Match.ParentName != "" && rule.Match.ParentNameType == types.MatchTypeExact {
		m.exactParentNameRules[rule.Match.ParentName] = append(
			m.exactParentNameRules[rule.Match.ParentName], rule)
		indexed = true
	}
	if !indexed || rule.Match.ProcessNameType == types.MatchTypeContains || rule.Match.ParentNameType == types.MatchTypeContains {
		m.partialMatchRules = append(m.partialMatchRules, rule)
	}
}

func (m *execMatcher) Match(event events.ProcessedEvent) (matched bool, rule *types.Rule, allowed bool) {
	return filterRulesByAction(m.getCandidateRules(event), m.matchRuleWrapper, event)
}

func (m *execMatcher) matchRuleWrapper(rule *types.Rule, event events.ProcessedEvent) bool {
	return m.matchRule(rule, event)
}

func (m *execMatcher) CollectAlerts(event events.ProcessedEvent) []types.MatchedAlert {
	candidates := m.getCandidateRules(event)
	for _, rule := range candidates {
		if rule.Action == types.ActionAllow && m.matchRule(rule, event) {
			return nil
		}
	}
	seen := make(map[*types.Rule]bool)
	var alerts []types.MatchedAlert
	for _, rule := range candidates {
		if seen[rule] || rule.Action == types.ActionAllow {
			continue
		}
		seen[rule] = true
		if m.matchRule(rule, event) {
			alerts = append(alerts, types.MatchedAlert{Rule: *rule, Event: event, Message: rule.Description})
		}
	}
	return alerts
}

func (m *execMatcher) getCandidateRules(event events.ProcessedEvent) []*types.Rule {
	var candidates []*types.Rule
	if rules, ok := m.exactProcessNameRules[event.Process]; ok {
		candidates = append(candidates, rules...)
	}
	if rules, ok := m.exactParentNameRules[event.Parent]; ok {
		candidates = append(candidates, rules...)
	}
	candidates = append(candidates, m.partialMatchRules...)
	return candidates
}

func (m *execMatcher) matchRule(rule *types.Rule, event events.ProcessedEvent) bool {
	match := rule.Match
	return (match.ProcessName == "" || matchString(event.Process, match.ProcessName, match.ProcessNameType)) &&
		(match.ParentName == "" || matchString(event.Parent, match.ParentName, match.ParentNameType)) &&
		matchPID(match.PID, event.Event.PID) &&
		(match.PPID == 0 || event.Event.PPID == match.PPID) &&
		matchCgroupID(match.CgroupID, event.Event.CgroupID)
}
