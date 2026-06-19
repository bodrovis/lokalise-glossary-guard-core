package checks

import (
	"errors"
	"sort"
	"strings"
	"sync"
)

var (
	mu     sync.RWMutex
	byName = map[string]CheckUnit{}
)

// Lookup returns a registered check by its case-insensitive name.
func Lookup(name string) (CheckUnit, bool) {
	name = normalizeName(name)

	mu.RLock()
	c, ok := byName[name]
	mu.RUnlock()

	return c, ok
}

// Register adds or replaces a check in the registry.
// It returns replaced=true if a check with the same normalized name already existed.
func Register(c CheckUnit) (bool, error) {
	if c == nil {
		return false, errors.New("checks.Register: nil check")
	}

	name := normalizeName(c.Name())
	if name == "" {
		return false, errors.New("checks.Register: empty name")
	}

	mu.Lock()
	_, existed := byName[name]
	byName[name] = c
	mu.Unlock()

	return existed, nil
}

// List returns a snapshot of all registered checks in unspecified order.
func List() []CheckUnit {
	return registrySnapshot()
}

// ListSorted returns all registered checks sorted by Priority asc, then Name asc.
func ListSorted() []CheckUnit {
	out := registrySnapshot()

	sort.Slice(out, func(i, j int) bool {
		pi, pj := out[i].Priority(), out[j].Priority()
		if pi != pj {
			return pi < pj
		}

		ki := normalizeName(out[i].Name())
		kj := normalizeName(out[j].Name())
		if ki != kj {
			return ki < kj
		}

		return out[i].Name() < out[j].Name()
	})

	return out
}

// Reset clears the registry. It is intended for tests.
func Reset() {
	mu.Lock()
	byName = map[string]CheckUnit{}
	mu.Unlock()
}

func registrySnapshot() []CheckUnit {
	mu.RLock()
	defer mu.RUnlock()

	out := make([]CheckUnit, 0, len(byName))
	for _, c := range byName {
		out = append(out, c)
	}

	return out
}

func normalizeName(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}
