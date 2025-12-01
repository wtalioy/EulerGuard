package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"eulerguard/pkg/config"
)

type OpenAIProvider struct {
	endpoint string
	apiKey   string
	model    string
	client   *http.Client
}

func NewOpenAIProvider(opts config.OpenAIOptions) *OpenAIProvider {
	apiKey := opts.APIKey
	if apiKey == "" {
		apiKey = os.Getenv("EULERGUARD_AI_API_KEY")
	}

	return &OpenAIProvider{
		endpoint: opts.Endpoint,
		apiKey:   apiKey,
		model:    opts.Model,
		client: &http.Client{
			Timeout: time.Duration(opts.Timeout) * time.Second,
		},
	}
}

func (o *OpenAIProvider) Name() string  { return "Cloud AI" }
func (o *OpenAIProvider) IsLocal() bool { return false }

func (o *OpenAIProvider) SingleChat(ctx context.Context, prompt string) (string, error) {
	messages := []Message{
		{Role: "system", Content: DiagnosisSystemPrompt},
		{Role: "user", Content: prompt},
	}
	return o.MultiChat(ctx, messages)
}

func (o *OpenAIProvider) MultiChat(ctx context.Context, messages []Message) (string, error) {
	openaiMessages := make([]map[string]string, len(messages))
	for i, msg := range messages {
		openaiMessages[i] = map[string]string{
			"role":    msg.Role,
			"content": msg.Content,
		}
	}

	reqBody := map[string]any{
		"model":       o.model,
		"messages":    openaiMessages,
		"temperature": 0.4,
		"max_tokens":  2048,
	}

	body, _ := json.Marshal(reqBody)
	req, err := http.NewRequestWithContext(ctx, "POST",
		o.endpoint+"/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+o.apiKey)

	resp, err := o.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("API request failed: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}
	if result.Error.Message != "" {
		return "", fmt.Errorf("API error: %s", result.Error.Message)
	}
	if len(result.Choices) == 0 {
		return "", fmt.Errorf("no response from API")
	}

	return result.Choices[0].Message.Content, nil
}

func (o *OpenAIProvider) CheckHealth(ctx context.Context) error {
	if o.apiKey == "" {
		return fmt.Errorf("API key not configured")
	}
	return nil
}
