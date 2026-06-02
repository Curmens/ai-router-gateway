package plugin

import (
	"errors"
	"sync"
)

var (
	registry   = make(map[string]ExternalTool)
	registryMu sync.RWMutex
)

func RegisterTool(name string, tool ExternalTool) {
	registryMu.Lock()
	defer registryMu.Unlock()
	registry[name] = tool
}

func GetTool(name string) (ExternalTool, error) {
	registryMu.RLock()
	defer registryMu.RUnlock()

	t, ok := registry[name]
	if !ok {
		return nil, errors.New("tool not whitelisted or found: " + name)
	}
	return t, nil
}
