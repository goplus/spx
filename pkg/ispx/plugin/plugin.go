package plugin

import (
	"log"
	"sync"

	"github.com/goplus/ixgo"
)

type Plugin interface {
	RegisterJS()
	RegisterPatch(ctx *ixgo.Context) error
	Init()
}

var pluginManager = NewPluginManager()

func Register(name string, plugin Plugin) {
	pluginManager.RegisterPlugin(name, plugin)
}

func GetPluginManager() *PluginManager {
	return pluginManager
}

type PluginManager struct {
	mu      sync.RWMutex
	plugins map[string]Plugin
}

func NewPluginManager() *PluginManager {
	return &PluginManager{
		plugins: make(map[string]Plugin),
	}
}

func (m *PluginManager) RegisterPlugin(name string, plugin Plugin) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.plugins[name]; exists {
		log.Printf("Plugin %s already registered, replacing", name)
	}
	m.plugins[name] = plugin
	log.Println("Registered plugin:", name)
}

func (m *PluginManager) RegisterJS() {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, plugin := range m.plugins {
		plugin.RegisterJS()
	}
}

func (m *PluginManager) RegisterPatch(ctx *ixgo.Context) error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for name, plugin := range m.plugins {
		if err := plugin.RegisterPatch(ctx); err != nil {
			log.Printf("Plugin %s RegisterPatch Failed: %v", name, err)
			return err
		}
	}
	return nil
}

func (m *PluginManager) Init() {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, plugin := range m.plugins {
		plugin.Init()
	}
}
