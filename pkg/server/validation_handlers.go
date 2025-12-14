package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"aegis/pkg/rules"
	"aegis/pkg/types"
)

// ValidationResponse contains validation data for a rule
type ValidationResponse struct {
	Rule       *types.Rule              `json:"rule"`
	Validation rules.PromotionReadiness `json:"validation"`
	Stats      rules.TestingStats       `json:"stats"`
}

// TestingRuleResponse contains testing rule data with validation
type TestingRuleResponse struct {
	Rule       *types.Rule              `json:"rule"`
	Validation rules.PromotionReadiness `json:"validation"`
	Stats      rules.TestingStats       `json:"stats"`
}

// GetRuleValidationHandler returns validation status for a rule
func (a *App) GetRuleValidationHandler(w http.ResponseWriter, r *http.Request) {
	// Extract rule ID from URL path
	pathParts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/rules/validation/"), "/")
	if len(pathParts) == 0 {
		http.Error(w, "Invalid rule name", http.StatusBadRequest)
		return
	}
	ruleID := pathParts[0]
	// URL decode the rule name
	if decoded, err := url.QueryUnescape(ruleID); err == nil {
		ruleID = decoded
	}

	// Get rule from engine
	rule, _ := a.findRuleByName(ruleID)
	if rule == nil {
		http.Error(w, "Rule not found", http.StatusNotFound)
		return
	}

	// Get validation status
	testingBuffer := a.getTestingBuffer()
	if testingBuffer == nil {
		http.Error(w, "testing buffer not available", http.StatusInternalServerError)
		return
	}
	opts := a.Options()
	validationService := rules.NewValidationService(testingBuffer, opts.PromotionMinObservationMinutes, opts.PromotionMinHits)
	validation := validationService.CalculatePromotionReadiness(rule)

	// Get testing stats
	testingStats := testingBuffer.GetStats(rule.Name)

	response := ValidationResponse{
		Rule:       rule,
		Validation: validation,
		Stats:      testingStats,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// PromoteRule promotes a rule from testing to production mode
func (a *App) PromoteRuleHandler(w http.ResponseWriter, r *http.Request) {
	// Extract rule ID from URL path
	pathParts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/rules/validation/"), "/")
	if len(pathParts) < 2 {
		http.Error(w, "Invalid rule name", http.StatusBadRequest)
		return
	}
	ruleID := pathParts[0]

	// Get rule from engine
	rule, _ := a.findRuleByName(ruleID)
	if rule == nil {
		http.Error(w, "Rule not found", http.StatusNotFound)
		return
	}

	if !rule.IsTesting() {
		http.Error(w, "Rule must be in testing mode to promote", http.StatusBadRequest)
		return
	}

	// Parse request body for force flag
	var reqBody struct {
		Force bool `json:"force"`
	}
	if r.Body != nil {
		json.NewDecoder(r.Body).Decode(&reqBody)
	}

	// Check readiness (unless force is true)
	if !reqBody.Force {
		testingBuffer := a.getTestingBuffer()
		if testingBuffer == nil {
			http.Error(w, "testing buffer not available", http.StatusInternalServerError)
			return
		}
		opts := a.Options()
	validationService := rules.NewValidationService(testingBuffer, opts.PromotionMinObservationMinutes, opts.PromotionMinHits)
		readiness := validationService.CalculatePromotionReadiness(rule)
		if !readiness.IsReady {
			http.Error(w, fmt.Sprintf("Rule is not ready for promotion. Score: %.0f%%. Use force=true to promote anyway.", readiness.Score*100), http.StatusBadRequest)
			return
		}
	}

	// Promote rule using existing API (updates YAML and reloads)
	// PromoteRule will set State, Mode, and PromotedAt
	if err := a.PromoteRule(ruleID); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"success": true, "promoted": ruleID})
}

// GetTestingRules returns all testing rules with validation data
func (a *App) GetTestingRules(w http.ResponseWriter, r *http.Request) {
	allRules := a.getRulesInternal()

	// Filter to testing rules only
	var testingRules []*types.Rule
	for i := range allRules {
		// Check both IsTesting() and explicit state comparison
		if allRules[i].IsTesting() || allRules[i].State == types.RuleStateTesting {
			testingRules = append(testingRules, &allRules[i])
		}
	}

	// Enrich with validation data
	response := make([]TestingRuleResponse, len(testingRules))
	testingBuffer := a.getTestingBuffer()
	if testingBuffer == nil {
		http.Error(w, "testing buffer not available", http.StatusInternalServerError)
		return
	}
	opts := a.Options()
	validationService := rules.NewValidationService(testingBuffer, opts.PromotionMinObservationMinutes, opts.PromotionMinHits)

	for i, rule := range testingRules {
		validation := validationService.CalculatePromotionReadiness(rule)
		stats := testingBuffer.GetStats(rule.Name)

		response[i] = TestingRuleResponse{
			Rule:       rule,
			Validation: validation,
			Stats:      stats,
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// DemoteRule demotes a rule from production to testing mode
func (a *App) DemoteRule(w http.ResponseWriter, r *http.Request) {
	// Extract rule ID from URL path
	pathParts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/rules/validation/"), "/")
	if len(pathParts) < 2 {
		http.Error(w, "Invalid rule name", http.StatusBadRequest)
		return
	}
	ruleID := pathParts[0]

	// Get rule from engine
	// Find rule by name in current rules
	if a.core == nil || a.core.RuleEngine == nil {
		http.Error(w, "rule engine not available", http.StatusInternalServerError)
		return
	}
	var rule *types.Rule
	allRules := a.core.RuleEngine.GetRules()
	for i := range allRules {
		if allRules[i].Name == ruleID {
			rule = &allRules[i]
			break
		}
	}
	if rule == nil {
		http.Error(w, "Rule not found", http.StatusNotFound)
		return
	}

	if !rule.IsProduction() {
		http.Error(w, "Rule must be in production mode to demote", http.StatusBadRequest)
		return
	}

	// Demote rule
	now := time.Now()
	rule.State = types.RuleStateTesting
	rule.DeployedAt = &now
	rule.ActualTestingHits = 0 // Reset testing hit count

	// Persist changes and reload
	if err := a.saveAndReloadRules(allRules); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Clear testing hits for this rule
	if testingBuffer := a.getTestingBuffer(); testingBuffer != nil {
		testingBuffer.ClearHits(ruleID)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"success": true, "demoted": ruleID})
}
