package rules

import (
	"eulerguard/pkg/events"
	"eulerguard/pkg/types"
)

type Engine struct {
	rules          []types.Rule
	execMatcher    *execMatcher
	fileMatcher    *fileMatcher
	connectMatcher *connectMatcher
}

func NewEngine(rules []types.Rule) *Engine {
	for i := range rules {
		rules[i].Match.Prepare()
	}
	return &Engine{
		rules:          rules,
		execMatcher:    newExecMatcher(rules),
		fileMatcher:    newFileMatcher(rules),
		connectMatcher: newConnectMatcher(rules),
	}
}

func (e *Engine) MatchExec(event events.ProcessedEvent) (matched bool, rule *types.Rule, allowed bool) {
	if e.execMatcher == nil {
		return false, nil, false
	}
	return e.execMatcher.Match(event)
}

func (e *Engine) CollectExecAlerts(event events.ProcessedEvent) []types.MatchedAlert {
	if e.execMatcher == nil {
		return nil
	}
	return e.execMatcher.CollectAlerts(event)
}

func (e *Engine) MatchFile(ino, dev uint64, filename string, pid uint32, cgroupID uint64) (matched bool, rule *types.Rule, allowed bool) {
	if e.fileMatcher == nil {
		return false, nil, false
	}
	return e.fileMatcher.Match(ino, dev, filename, pid, cgroupID)
}

func (e *Engine) MatchConnect(event *events.ConnectEvent) (matched bool, rule *types.Rule, allowed bool) {
	if e.connectMatcher == nil {
		return false, nil, false
	}
	return e.connectMatcher.Match(event)
}

func (e *Engine) GetRules() []types.Rule {
	return e.rules
}
