package ai

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"aegis/pkg/ai/prompt"
	"aegis/pkg/rules"
	"aegis/pkg/storage"
	"aegis/pkg/types"
	"gopkg.in/yaml.v3"
)

// RuleGenRequest represents a rule generation request.
type RuleGenRequest struct {
	Description string          `json:"description"` // 自然语言描述
	Context     *RequestContext `json:"context"`     // 上下文
	Examples    []types.Rule    `json:"examples"`   // 现有规则作为参考
}

// RuleGenResponse contains the generated rule and metadata.
type RuleGenResponse struct {
	Rule       types.Rule                    `json:"rule"`       // 生成的规则
	YAML       string                        `json:"yaml"`       // YAML 格式
	Reasoning  string                        `json:"reasoning"`  // AI 推理过程
	Confidence float64                       `json:"confidence"` // 置信度
	Warnings   []string                      `json:"warnings"`   // 潜在风险警告
}

// GenerateRule generates a security rule from natural language description.
func (s *Service) GenerateRule(ctx context.Context, req *RuleGenRequest, ruleEngine *rules.Engine, store storage.EventStore) (*RuleGenResponse, error) {
	if !s.IsEnabled() {
		return nil, fmt.Errorf("AI service is not available")
	}

	// Build prompt (simplified - would use template engine in production)
	examplesYAML := ""
	for _, ex := range req.Examples {
		yb, _ := yaml.Marshal(ex)
		examplesYAML += string(yb) + "\n---\n"
	}

	userPrompt := fmt.Sprintf("User request: \"%s\"\n\nExisting rules (examples):\n%s\n\nGenerate rule:", req.Description, examplesYAML)
	systemPrompt := prompt.RuleGenSystemPrompt

	// Call AI service
	fullPrompt := systemPrompt + "\n\n" + userPrompt
	response, err := s.provider.SingleChat(ctx, fullPrompt)
	if err != nil {
		return nil, fmt.Errorf("AI inference failed: %w", err)
	}

	// Extract YAML from response
	ruleYAML := extractYAMLFromResponse(response)
	if ruleYAML == "" {
		return nil, fmt.Errorf("failed to extract rule YAML from AI response")
	}

	// Parse YAML to Rule
	var rule types.Rule
	if err := yaml.Unmarshal([]byte(ruleYAML), &rule); err != nil {
		return nil, fmt.Errorf("failed to parse generated rule YAML: %w", err)
	}

	// Generate YAML representation (without metadata fields)
	cleanRule := rules.CleanRuleForYAML(rule)
	yamlBytes, err := yaml.Marshal(cleanRule)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal rule to YAML: %w", err)
	}

	// Extract reasoning and warnings from response
	reasoning, warnings := extractReasoningAndWarnings(response)

	resp := &RuleGenResponse{
		Rule:       rule,
		YAML:       string(yamlBytes),
		Reasoning:  reasoning,
		Confidence: 0.8, // Default confidence
		Warnings:   warnings,
	}

	return resp, nil
}

// extractYAMLFromResponse tries to extract a YAML rule from a free-form LLM response.
// It prefers fenced code blocks (```yaml ... ```). If none found, it heuristically
// extracts lines starting from the first occurrence of common rule keys.
func extractYAMLFromResponse(text string) string {
	if text == "" {
		return ""
	}

	// 1) Try fenced code block: ```yaml ... ``` (case-insensitive for yaml/yml or without label)
	re := regexp.MustCompile("(?s)```(?i:(?:yaml|yml))?\\s*(.*?)```")
	if m := re.FindStringSubmatch(text); len(m) == 2 {
		return strings.TrimSpace(m[1])
	}

	// 2) Try to find a block that looks like YAML by scanning from a key line
	lines := strings.Split(text, "\n")
	start := -1
	for i, ln := range lines {
		l := strings.TrimSpace(ln)
		if strings.HasPrefix(l, "name:") || strings.HasPrefix(l, "match:") || strings.HasPrefix(l, "action:") {
			start = i
			break
		}
	}
	if start >= 0 {
		var b strings.Builder
		for i := start; i < len(lines); i++ {
			l := lines[i]
			// Stop on obvious section headers from the explanation
			trim := strings.TrimSpace(l)
			if strings.HasPrefix(trim, "Reasoning:") || strings.HasPrefix(trim, "Warnings:") || strings.HasPrefix(trim, "---") {
				break
			}
			b.WriteString(l)
			b.WriteString("\n")
		}
		return strings.TrimSpace(b.String())
	}

	// 3) Fallback: if the entire text seems to start with YAML-ish keys, return it
	trim := strings.TrimSpace(text)
	if strings.HasPrefix(trim, "name:") || strings.HasPrefix(trim, "match:") || strings.HasPrefix(trim, "action:") {
		return trim
	}

	// 4) Nothing found
	return ""
}

// extractReasoningAndWarnings extracts reasoning and warnings from AI response.
func extractReasoningAndWarnings(text string) (string, []string) {
	// Simple extraction - look for "Reasoning:" and "Warnings:" sections
	reasoning := ""
	warnings := []string{}

	// This is a simplified version - in production, use more sophisticated parsing
	// For now, return the full response as reasoning
	reasoning = text

	return reasoning, warnings
}

