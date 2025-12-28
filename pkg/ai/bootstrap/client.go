package bootstrap

import (
	"fmt"

	"aegis/pkg/ai/providers"
	"aegis/pkg/ai/service"
	"aegis/pkg/config"
)

func NewClientFromConfig(opts config.AIOptions) (*service.Service, error) {
	var provider providers.Provider

	switch opts.Mode {
	case "ollama":
		provider = providers.NewOllamaProvider(opts.Ollama)
	case "openai":
		provider = providers.NewOpenAIProvider(opts.OpenAI)
	default:
		return nil, fmt.Errorf("unknown AI mode: %s", opts.Mode)
	}

	return service.NewClient(provider), nil
}
