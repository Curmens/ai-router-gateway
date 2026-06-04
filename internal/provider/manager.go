package provider

import (
	"errors"
	"sync"

	"github.com/user1024/auto-router/internal/config"
)

var (
	Registry   = make(map[string]ChatProvider)
	registryMu sync.RWMutex
)

func InitProviders(cfg *config.Config) {
	registryMu.Lock()
	defer registryMu.Unlock()

	// Rebuild from scratch so providers disabled at runtime are dropped.
	Registry = make(map[string]ChatProvider)

	if cfg.Providers.OpenAI.Enabled {
		Registry["openai"] = NewOpenAIProvider(cfg.Providers.OpenAI)
	}
	if cfg.Providers.Gemini.Enabled {
		Registry["gemini"] = NewGeminiProvider(cfg.Providers.Gemini)
	}
	if cfg.Providers.Ollama.Enabled {
		Registry["ollama"] = NewOllamaProvider(cfg.Providers.Ollama)
	}
	if cfg.Providers.Subscription.Enabled {
		Registry["subscription"] = NewClaudeProvider(cfg.Providers.Subscription)
	}
	if cfg.Providers.Agy.Enabled {
		Registry["agy"] = NewAgyProvider(cfg.Providers.Agy)
	}
}

func GetProvider(name string) (ChatProvider, error) {
	registryMu.RLock()
	defer registryMu.RUnlock()

	p, ok := Registry[name]
	if !ok {
		return nil, errors.New("provider not found: " + name)
	}
	return p, nil
}

func GetEnabledProviders() []ChatProvider {
	registryMu.RLock()
	defer registryMu.RUnlock()

	list := make([]ChatProvider, 0, len(Registry))
	for _, p := range Registry {
		list = append(list, p)
	}
	return list
}
