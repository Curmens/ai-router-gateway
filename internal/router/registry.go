package router

import (
	"sync"

	"github.com/user1024/auto-router/internal/config"
)

type RegistryModel struct {
	Name                string
	Provider            string
	CostPer1kPrompt     float64
	CostPer1kCompletion float64
}

var (
	modelRegistry = make(map[string]RegistryModel)
	registryMu    sync.RWMutex
)

func InitRegistry(cfg *config.Config) {
	registryMu.Lock()
	defer registryMu.Unlock()

	modelRegistry = make(map[string]RegistryModel)

	for _, m := range cfg.Providers.OpenAI.Models {
		modelRegistry[m.Name] = RegistryModel{
			Name:                m.Name,
			Provider:            "openai",
			CostPer1kPrompt:     m.CostPer1kPrompt,
			CostPer1kCompletion: m.CostPer1kCompletion,
		}
	}
	for _, m := range cfg.Providers.Gemini.Models {
		modelRegistry[m.Name] = RegistryModel{
			Name:                m.Name,
			Provider:            "gemini",
			CostPer1kPrompt:     m.CostPer1kPrompt,
			CostPer1kCompletion: m.CostPer1kCompletion,
		}
	}
	for _, m := range cfg.Providers.Ollama.Models {
		modelRegistry[m.Name] = RegistryModel{
			Name:                m.Name,
			Provider:            "ollama",
			CostPer1kPrompt:     m.CostPer1kPrompt,
			CostPer1kCompletion: m.CostPer1kCompletion,
		}
	}
	for _, m := range cfg.Providers.Subscription.Models {
		modelRegistry[m.Name] = RegistryModel{
			Name:                m.Name,
			Provider:            "subscription",
			CostPer1kPrompt:     m.CostPer1kPrompt,
			CostPer1kCompletion: m.CostPer1kCompletion,
		}
	}
	for _, m := range cfg.Providers.Agy.Models {
		modelRegistry[m.Name] = RegistryModel{
			Name:                m.Name,
			Provider:            "agy",
			CostPer1kPrompt:     m.CostPer1kPrompt,
			CostPer1kCompletion: m.CostPer1kCompletion,
		}
	}
}

func GetRegistryModel(name string) (RegistryModel, bool) {
	registryMu.RLock()
	defer registryMu.RUnlock()
	m, ok := modelRegistry[name]
	return m, ok
}

func GetAllRegistryModels() []RegistryModel {
	registryMu.RLock()
	defer registryMu.RUnlock()

	list := make([]RegistryModel, 0, len(modelRegistry))
	for _, m := range modelRegistry {
		list = append(list, m)
	}
	return list
}
