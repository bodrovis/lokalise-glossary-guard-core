package checks

import (
	"errors"
	"sort"
	"strings"
	"sync"
)

// thread-safe in-memory registry: name -> CheckUnit
var (
	mu     sync.RWMutex
	byName = map[string]CheckUnit{}
)

// Lookup returns a registered check by its (case-insensitive) name.
func Lookup(name string) (CheckUnit, bool) {
	name = normalizeName(name)
	mu.RLock()
	c, ok := byName[name]
	mu.RUnlock()
	return c, ok
}

// Register adds or replaces a check in the registry.
// Returns replaced=true if a check with the same normalized name already existed.
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

// List returns a snapshot of all registered checks (unsorted).
func List() []CheckUnit {
	mu.RLock()
	out := make([]CheckUnit, 0, len(byName))
	for _, c := range byName {
		out = append(out, c)
	}
	mu.RUnlock()
	return out
}

// ListSorted returns all registered checks sorted by Priority asc, then Name asc.
func ListSorted() []CheckUnit {
	mu.RLock()
	out := make([]CheckUnit, 0, len(byName))
	for _, c := range byName {
		out = append(out, c)
	}
	mu.RUnlock()

	sort.Slice(out, func(i, j int) bool {
		pi, pj := out[i].Priority(), out[j].Priority()
		if pi != pj {
			return pi < pj
		}
		// tie-break by normalized name
		ki := strings.ToLower(strings.TrimSpace(out[i].Name()))
		kj := strings.ToLower(strings.TrimSpace(out[j].Name()))
		if ki != kj {
			return ki < kj
		}
		// final tie-breaker (just in case)
		return out[i].Name() < out[j].Name()
	})

	return out
}

// Reset clears the registry.
func Reset() {
	mu.Lock()
	byName = map[string]CheckUnit{}
	mu.Unlock()
}

// normalizeName trims and lowercases a check name.
func normalizeName(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}
