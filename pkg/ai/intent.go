package ai

import (
	"context"
	"encoding/json"
	"fmt"

	"aegis/pkg/ai/prompt"
)

// IntentType represents the type of user intent.
type IntentType string

const (
	IntentCreateRule   IntentType = "create_rule"   // 创建规则
	IntentQueryEvents  IntentType = "query_events"  // 查询事件
	IntentExplainEvent IntentType = "explain_event" // 解释事件
	IntentAnalyzeProc  IntentType = "analyze_process" // 分析进程
	IntentPromoteRule  IntentType = "promote_rule"  // 转正规则
	IntentNavigation   IntentType = "navigation"    // 导航到页面
)

// Intent represents a parsed user intent.
type Intent struct {
	Type       IntentType              `json:"type"`       // create_rule, query_events, explain, analyze, etc.
	Confidence float64                 `json:"confidence"` // 0-1 置信度
	Params     map[string]interface{} `json:"params"`     // 意图参数（结构化）
	Preview    *Preview                `json:"preview"`    // 预览（如生成的规则 YAML）
	Warnings   []string                `json:"warnings"`   // AI 警告
	Ambiguous  bool                    `json:"ambiguous"` // 是否模糊
	Clarification string               `json:"clarification"` // 如果需要澄清，要问什么
}

// Preview contains a preview of the intent result (e.g., generated rule YAML).
type Preview struct {
	Type    string `json:"type"`    // "rule", "query", etc.
	Content string `json:"content"` // Preview content (YAML, JSON, etc.)
}

// RequestContext contains the current request context for intent parsing.
type RequestContext struct {
	CurrentPage   string   `json:"current_page"`   // Current page name
	SelectedItem  string   `json:"selected_item"`  // Selected event/rule ID
	RecentActions []string `json:"recent_actions"` // Recent user actions
}

// ParseIntent parses user's natural language input and extracts structured intent.
func (s *Service) ParseIntent(ctx context.Context, input string, reqCtx *RequestContext) (*Intent, error) {
	if !s.IsEnabled() {
		return nil, fmt.Errorf("AI service is not available")
	}

	// Build prompt context
	promptCtx := &prompt.PromptContext{
		Input:         input,
		CurrentPage:   reqCtx.CurrentPage,
		SelectedItem:  reqCtx.SelectedItem,
		RecentActions: reqCtx.RecentActions,
	}

	// Generate prompt using template
	userPrompt := buildIntentPrompt(promptCtx)
	systemPrompt := prompt.IntentSystemPrompt

	// Call AI service
	fullPrompt := systemPrompt + "\n\n" + userPrompt
	response, err := s.provider.SingleChat(ctx, fullPrompt)
	if err != nil {
		return nil, fmt.Errorf("AI service error: %w", err)
	}

	// Parse JSON response
	var intent Intent
	if err := json.Unmarshal([]byte(response), &intent); err != nil {
		// If JSON parsing fails, try to extract JSON from markdown code blocks
		cleaned := extractJSONFromMarkdown(response)
		if err := json.Unmarshal([]byte(cleaned), &intent); err != nil {
			return nil, fmt.Errorf("failed to parse intent JSON: %w", err)
		}
	}

	// Validate intent
	if intent.Type == "" {
		return nil, fmt.Errorf("intent type is empty")
	}

	// Set default confidence if not provided
	if intent.Confidence == 0 {
		intent.Confidence = 0.5 // Default low confidence
	}

	return &intent, nil
}

// buildIntentPrompt builds the user prompt for intent parsing.
func buildIntentPrompt(ctx *prompt.PromptContext) string {
	// Simple template-based prompt (can be enhanced with actual template engine)
	return fmt.Sprintf(`Current context:
- Page: %s
- Selected: %s
- Recent actions: %v

User input: "%s"

Parse the intent:`, ctx.CurrentPage, ctx.SelectedItem, ctx.RecentActions, ctx.Input)
}

// extractJSONFromMarkdown extracts JSON from markdown code blocks.
func extractJSONFromMarkdown(text string) string {
	// Try to find JSON in code blocks
	start := -1
	end := -1
	
	for i := 0; i < len(text)-3; i++ {
		if text[i:i+3] == "```" {
			if start == -1 {
				start = i + 3
				// Skip language identifier if present
				if start < len(text) && text[start] != '\n' {
					for start < len(text) && text[start] != '\n' {
						start++
					}
				}
				if start < len(text) {
					start++ // Skip newline
				}
			} else {
				end = i
				break
			}
		}
	}
	
	if start > 0 && end > start {
		return text[start:end]
	}
	
	// Try to find JSON object directly
	start = -1
	for i := 0; i < len(text); i++ {
		if text[i] == '{' {
			start = i
			break
		}
	}
	
	if start >= 0 {
		// Find matching closing brace
		braceCount := 0
		for i := start; i < len(text); i++ {
			if text[i] == '{' {
				braceCount++
			} else if text[i] == '}' {
				braceCount--
				if braceCount == 0 {
					return text[start : i+1]
				}
			}
		}
	}
	
	return text // Return original if extraction fails
}

