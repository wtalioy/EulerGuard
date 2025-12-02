package rules

import (
	"eulerguard/pkg/events"
	"eulerguard/pkg/types"
	"eulerguard/pkg/utils"
)

type connectMatcher struct {
	rules []*types.Rule
}

func newConnectMatcher(rules []types.Rule) *connectMatcher {
	matcher := &connectMatcher{rules: make([]*types.Rule, 0)}
	for i := range rules {
		if rules[i].Match.DestPort != 0 || rules[i].Match.DestIP != "" {
			matcher.rules = append(matcher.rules, &rules[i])
		}
	}
	return matcher
}

func (m *connectMatcher) Match(event *events.ConnectEvent) (matched bool, rule *types.Rule, allowed bool) {
	return filterRulesByAction(m.rules, m.matchRule, event)
}

func (m *connectMatcher) matchRule(rule *types.Rule, event *events.ConnectEvent) bool {
	match := rule.Match
	if match.DestPort == 0 && match.DestIP == "" {
		return false
	}
	if match.DestPort != 0 && event.Port != match.DestPort {
		return false
	}
	if match.DestIP != "" {
		if eventIP := utils.ExtractIP(event); eventIP == "" || !match.MatchIP(eventIP) {
			return false
		}
	}
	return matchCgroupID(match.CgroupID, event.CgroupID) && matchPID(match.PID, event.PID)
}
