package rules

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"eulerguard/pkg/events"
	"eulerguard/pkg/types"
)

func TestMatchWithAllow_AllowRuleSuppressesAlerts(t *testing.T) {
	rules := []types.Rule{
		{
			Name:     "Alert on bash",
			Severity: "high",
			Action:   types.ActionAlert,
			Match: types.MatchCondition{
				ProcessName:     "bash",
				ProcessNameType: types.MatchTypeExact,
			},
		},
		{
			Name:     "Allow bash from sshd",
			Severity: "info",
			Action:   types.ActionAllow,
			Match: types.MatchCondition{
				ProcessName:     "bash",
				ProcessNameType: types.MatchTypeExact,
				ParentName:      "sshd",
				ParentNameType:  types.MatchTypeExact,
			},
		},
	}

	engine := NewEngine(rules)

	// Test case 1: bash from sshd should be allowed (no alerts)
	event1 := events.ProcessedEvent{
		Process: "bash",
		Parent:  "sshd",
	}
	matched, _, allowed := engine.MatchExec(event1)
	if !matched || !allowed {
		t.Error("Expected event to be matched and allowed")
	}
	alerts := engine.CollectExecAlerts(event1)
	if len(alerts) > 0 {
		t.Errorf("Expected no alerts when allowed, got %d", len(alerts))
	}

	// Test case 2: bash from unknown parent should trigger alert
	event2 := events.ProcessedEvent{
		Process: "bash",
		Parent:  "nginx",
	}
	matched2, _, allowed2 := engine.MatchExec(event2)
	if !matched2 || allowed2 {
		t.Error("Expected event to be matched but NOT allowed")
	}
	alerts2 := engine.CollectExecAlerts(event2)
	if len(alerts2) != 1 {
		t.Errorf("Expected 1 alert, got %d", len(alerts2))
	}
}

func TestMatchWithAllow_AllowRuleOrderIndependent(t *testing.T) {
	// Test that allow rules work regardless of order in the rules list
	rules := []types.Rule{
		// Allow rule BEFORE alert rule
		{
			Name:     "Allow bash from sshd",
			Severity: "info",
			Action:   types.ActionAllow,
			Match: types.MatchCondition{
				ProcessName:     "bash",
				ProcessNameType: types.MatchTypeExact,
				ParentName:      "sshd",
				ParentNameType:  types.MatchTypeExact,
			},
		},
		{
			Name:     "Alert on bash",
			Severity: "high",
			Action:   types.ActionAlert,
			Match: types.MatchCondition{
				ProcessName:     "bash",
				ProcessNameType: types.MatchTypeExact,
			},
		},
	}

	engine := NewEngine(rules)

	event := events.ProcessedEvent{
		Process: "bash",
		Parent:  "sshd",
	}
	matched, _, allowed := engine.MatchExec(event)
	if !matched || !allowed {
		t.Error("Expected event to be matched and allowed (allow rule before alert)")
	}
	alerts := engine.CollectExecAlerts(event)
	if len(alerts) > 0 {
		t.Errorf("Expected no alerts when allowed, got %d", len(alerts))
	}
}

func TestMatchFileWithAllow(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "critical.conf")
	if err := os.WriteFile(target, []byte("secret"), 0o644); err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}

	rules := []types.Rule{
		{
			Name:     "Alert on critical file",
			Severity: "warning",
			Action:   types.ActionAlert,
			Match: types.MatchCondition{
				Filename: target,
			},
		},
		{
			Name:     "Allow critical file access",
			Severity: "info",
			Action:   types.ActionAllow,
			Match: types.MatchCondition{
				Filename: target,
			},
		},
	}

	engine := NewEngine(rules)

	info, err := os.Stat(target)
	if err != nil {
		t.Fatalf("Failed to stat temp file: %v", err)
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		t.Fatal("Expected Stat_t for temp file")
	}

	// Allow rule should take precedence
	matched, rule, allowed := engine.MatchFile(stat.Ino, uint64(stat.Dev), target, 1234, 0)
	if !matched {
		t.Error("Expected match")
	}
	if !allowed {
		t.Error("Expected to be allowed")
	}
	if rule == nil {
		t.Error("Expected rule to be returned")
	}
}

func TestMatchConnectWithAllow(t *testing.T) {
	rules := []types.Rule{
		{
			Name:     "Alert on port 443",
			Severity: "info",
			Action:   types.ActionAlert,
			Match: types.MatchCondition{
				DestPort: 443,
			},
		},
		{
			Name:     "Allow port 443",
			Severity: "info",
			Action:   types.ActionAllow,
			Match: types.MatchCondition{
				DestPort: 443,
			},
		},
	}

	engine := NewEngine(rules)

	event := &events.ConnectEvent{
		Port: 443,
	}

	matched, rule, allowed := engine.MatchConnect(event)
	if !matched {
		t.Error("Expected match")
	}
	if !allowed {
		t.Error("Expected to be allowed")
	}
	if rule == nil {
		t.Error("Expected rule to be returned")
	}
}

func TestMergeRules(t *testing.T) {
	existing := []types.Rule{
		{
			Name:   "Existing Alert",
			Action: types.ActionAlert,
			Match: types.MatchCondition{
				ProcessName: "curl",
			},
		},
	}

	newRules := []types.Rule{
		{
			Name:   "New Allow",
			Action: types.ActionAllow,
			Match: types.MatchCondition{
				ProcessName: "bash",
				ParentName:  "sshd",
			},
		},
		// Duplicate of existing (should not be added)
		{
			Name:   "Duplicate",
			Action: types.ActionAlert,
			Match: types.MatchCondition{
				ProcessName: "curl",
			},
		},
	}

	merged := MergeRules(existing, newRules)

	if len(merged) != 2 {
		t.Errorf("Expected 2 rules after merge, got %d", len(merged))
	}

	// Check that original rule is preserved
	if merged[0].Name != "Existing Alert" {
		t.Error("Expected existing rule to be first")
	}

	// Check that new unique rule was added
	if merged[1].Name != "New Allow" {
		t.Error("Expected new allow rule to be added")
	}
}

func TestMultipleAlertsForSingleExecEvent(t *testing.T) {
	// Test case: single exec event violating multiple alert rules
	rules := []types.Rule{
		{
			Name:        "Alert on bash execution",
			Description: "bash shell execution detected",
			Severity:    "medium",
			Action:      types.ActionAlert,
			Match: types.MatchCondition{
				ProcessName:     "bash",
				ProcessNameType: types.MatchTypeExact,
			},
		},
		{
			Name:        "Alert on suspicious parent",
			Description: "process spawned from suspicious parent",
			Severity:    "high",
			Action:      types.ActionAlert,
			Match: types.MatchCondition{
				ParentName:     "wget",
				ParentNameType: types.MatchTypeExact,
			},
		},
		{
			Name:        "Alert on specific PID",
			Description: "process with specific PID 1234",
			Severity:    "low",
			Action:      types.ActionAlert,
			Match: types.MatchCondition{
				PID: 1234, // Exact PID match
			},
		},
	}

	engine := NewEngine(rules)

	// This exec event should match ALL THREE alert rules:
	// 1. ProcessName = "bash" (exact match)
	// 2. ParentName = "wget" (exact match)
	// 3. PID = 1234 (>= 1000)
	event := events.ProcessedEvent{
		Event: events.ExecEvent{
			PID:      1234,
			PPID:     1000,
			CgroupID: 0,
		},
		Process: "bash",
		Parent:  "wget",
	}

	matched, _, allowed := engine.MatchExec(event)
	if !matched || allowed {
		t.Error("Expected event to be matched but NOT allowed")
	}

	alerts := engine.CollectExecAlerts(event)
	if len(alerts) != 3 {
		t.Errorf("Expected 3 alerts, got %d", len(alerts))
		for i, alert := range alerts {
			t.Logf("Alert %d: %s (%s)", i+1, alert.Rule.Name, alert.Rule.Severity)
		}
	}

	// Verify each alert
	expectedRules := map[string]bool{
		"Alert on bash execution":    true,
		"Alert on suspicious parent": true,
		"Alert on specific PID":      true,
	}

	for _, alert := range alerts {
		if !expectedRules[alert.Rule.Name] {
			t.Errorf("Unexpected alert rule: %s", alert.Rule.Name)
		}
		delete(expectedRules, alert.Rule.Name)
	}

	if len(expectedRules) > 0 {
		t.Errorf("Missing alerts for rules: %v", expectedRules)
	}
}

func TestSaveAndLoadRules(t *testing.T) {
	rules := []types.Rule{
		{
			Name:        "Test Rule",
			Description: "A test rule",
			Severity:    "info",
			Action:      types.ActionAllow,
			Type:        types.RuleTypeExec,
			Match: types.MatchCondition{
				ProcessName:     "test",
				ProcessNameType: types.MatchTypeExact,
			},
		},
	}

	tmpFile := "/tmp/eulerguard_test_rules.yaml"

	// Save
	err := SaveRules(tmpFile, rules)
	if err != nil {
		t.Fatalf("Failed to save rules: %v", err)
	}

	// Load
	loaded, err := LoadRules(tmpFile)
	if err != nil {
		t.Fatalf("Failed to load rules: %v", err)
	}

	if len(loaded) != 1 {
		t.Fatalf("Expected 1 rule, got %d", len(loaded))
	}

	if loaded[0].Name != "Test Rule" {
		t.Errorf("Expected name 'Test Rule', got '%s'", loaded[0].Name)
	}

	if loaded[0].Action != types.ActionAllow {
		t.Errorf("Expected action 'allow', got '%s'", loaded[0].Action)
	}
}
