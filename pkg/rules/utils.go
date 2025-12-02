package rules

import (
	"strconv"
	"strings"

	"eulerguard/pkg/types"
)

// match a value against a pattern using the specified match type.
func matchString(value, pattern string, matchType types.MatchType) bool {
	switch matchType {
	case types.MatchTypeExact:
		return value == pattern
	case types.MatchTypePrefix:
		return strings.HasPrefix(value, pattern)
	case types.MatchTypeContains:
		return strings.Contains(value, pattern)
	default:
		return strings.Contains(value, pattern)
	}
}

func matchCgroupID(pattern string, cgroupID uint64) bool {
	return pattern == "" || strconv.FormatUint(cgroupID, 10) == pattern
}

func matchPID(pattern uint32, pid uint32) bool {
	return pattern == 0 || pid == pattern
}

// Returns: matched (any rule matched), rule (the matching rule), allowed (should the action be allowed)
func filterRulesByAction[T any](rules []*types.Rule, matchFn func(*types.Rule, T) bool, event T) (matched bool, rule *types.Rule, allowed bool) {
	var blockRule *types.Rule
	var alertRule *types.Rule

	for _, r := range rules {
		if !matchFn(r, event) {
			continue
		}
		switch r.Action {
		case types.ActionAllow:
			return true, r, true
		case types.ActionBlock:
			if blockRule == nil {
				blockRule = r
			}
		case types.ActionAlert:
			if alertRule == nil {
				alertRule = r
			}
		}
	}

	if blockRule != nil {
		return true, blockRule, false
	}
	if alertRule != nil {
		return true, alertRule, false
	}
	return false, nil, false
}
