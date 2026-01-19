// internal/chains/registry.go
package chains

import (
	"crypto-service/internal/domain"
	"fmt"
	"sync"
)

type Registry struct {
	chains map[string]domain.Chain
	mu     sync.RWMutex
}

func NewRegistry() *Registry {
	return &Registry{
		chains: make(map[string]domain.Chain),
	}
}

// Register adds a chain to registry
func (r *Registry) Register(chain domain.Chain) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.chains[chain.Name()] = chain
}

// Get retrieves a chain by name
func (r *Registry) Get(name string) (domain.Chain, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	chain, ok := r.chains[name]
	if !ok {
		return nil, fmt. Errorf("chain not supported: %s", name)
	}
	
	return chain, nil
}

// List returns all registered chains
func (r *Registry) List() []string {
	r.mu. RLock()
	defer r.mu.RUnlock()
	
	names := make([]string, 0, len(r.chains))
	for name := range r.chains {
		names = append(names, name)
	}
	
	return names
}