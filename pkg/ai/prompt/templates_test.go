package prompt

import (
	"testing"
)

func TestDefaultBudgets(t *testing.T) {
	if len(DefaultBudgets) != 4 {
		t.Errorf("Expected 4 budget types, got %d", len(DefaultBudgets))
	}

	intentBudget := DefaultBudgets["intent"]
	if intentBudget.SystemPrompt != 500 {
		t.Errorf("Expected intent system prompt budget 500, got %d", intentBudget.SystemPrompt)
	}

	rulegenBudget := DefaultBudgets["rulegen"]
	if rulegenBudget.Context != 500 {
		t.Errorf("Expected rulegen context budget 500, got %d", rulegenBudget.Context)
	}
}

func TestCompressContext(t *testing.T) {
	ctx := &PromptContext{
		CurrentPage:   "observatory",
		SelectedItem:  "event-123",
		RecentActions: []string{"viewed", "filtered"},
		Input:         "test input",
	}

	result := CompressContext(ctx, 100)
	if result == "" {
		t.Error("CompressContext returned empty string")
	}
}

func TestBatchAnalyze(t *testing.T) {
	items := []AnalysisItem{
		{Type: "process", ID: "123"},
		{Type: "process", ID: "456"},
		{Type: "workload", ID: "789"},
	}

	results := BatchAnalyze(items)
	if len(results) != len(items) {
		t.Errorf("Expected %d results, got %d", len(items), len(results))
	}

	for i, result := range results {
		if result.Item.Type != items[i].Type {
			t.Errorf("Result %d: expected type %s, got %s", i, items[i].Type, result.Item.Type)
		}
		if result.Item.ID != items[i].ID {
			t.Errorf("Result %d: expected ID %s, got %s", i, items[i].ID, result.Item.ID)
		}
	}
}

func TestPromptConstants(t *testing.T) {
	// Test that prompt constants are not empty
	if IntentSystemPrompt == "" {
		t.Error("IntentSystemPrompt is empty")
	}
	if IntentUserTemplate == "" {
		t.Error("IntentUserTemplate is empty")
	}
	if RuleGenSystemPrompt == "" {
		t.Error("RuleGenSystemPrompt is empty")
	}
	if RuleGenUserTemplate == "" {
		t.Error("RuleGenUserTemplate is empty")
	}
	if ExplainSystemPrompt == "" {
		t.Error("ExplainSystemPrompt is empty")
	}
	if ExplainUserTemplate == "" {
		t.Error("ExplainUserTemplate is empty")
	}
	if AnalyzeSystemPrompt == "" {
		t.Error("AnalyzeSystemPrompt is empty")
	}
	if AnalyzeUserTemplate == "" {
		t.Error("AnalyzeUserTemplate is empty")
	}
	if SentinelTestingPrompt == "" {
		t.Error("SentinelTestingPrompt is empty")
	}
	if SentinelAnomalyPrompt == "" {
		t.Error("SentinelAnomalyPrompt is empty")
	}
}
