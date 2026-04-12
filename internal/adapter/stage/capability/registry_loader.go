package capability

import (
	"embed"
	"fmt"
	"sync"

	domcap "github.com/mariotoffia/goagentmeta/internal/domain/capability"
	"gopkg.in/yaml.v3"
)

//go:embed target_registry/*.yaml
var registryFS embed.FS

// registryFileSpec maps target names to their embedded YAML filenames.
var registryFileSpec = map[string]string{
	"claude":  "target_registry/claude.yaml",
	"cursor":  "target_registry/cursor.yaml",
	"copilot": "target_registry/copilot.yaml",
	"codex":   "target_registry/codex.yaml",
}

// registryYAML is the YAML-deserializable shape of a target capability registry.
type registryYAML struct {
	Target   string            `yaml:"target"`
	Surfaces map[string]string `yaml:"surfaces"`
}

var (
	registryCacheMu sync.RWMutex
	registryCache   = make(map[string]*domcap.CapabilityRegistry)
)

// LoadRegistry returns the CapabilityRegistry for the given target.
// Registries are loaded once from embedded YAML and cached for the
// lifetime of the process.
func LoadRegistry(target string) (*domcap.CapabilityRegistry, error) {
	registryCacheMu.RLock()
	if r, ok := registryCache[target]; ok {
		registryCacheMu.RUnlock()
		return r, nil
	}
	registryCacheMu.RUnlock()

	registryCacheMu.Lock()
	defer registryCacheMu.Unlock()

	// Double-check after acquiring write lock.
	if r, ok := registryCache[target]; ok {
		return r, nil
	}

	filename, ok := registryFileSpec[target]
	if !ok {
		return nil, fmt.Errorf("no capability registry for target %q", target)
	}

	data, err := registryFS.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("reading registry %s: %w", filename, err)
	}

	var raw registryYAML
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parsing registry %s: %w", filename, err)
	}

	surfaces := make(map[string]domcap.SupportLevel, len(raw.Surfaces))
	for k, v := range raw.Surfaces {
		surfaces[k] = domcap.SupportLevel(v)
	}

	reg := &domcap.CapabilityRegistry{
		Target:   raw.Target,
		Surfaces: surfaces,
	}

	registryCache[target] = reg
	return reg, nil
}

// ResetCache clears the registry cache. Used in tests.
func ResetCache() {
	registryCacheMu.Lock()
	defer registryCacheMu.Unlock()
	registryCache = make(map[string]*domcap.CapabilityRegistry)
}

// AllRegistries loads and returns registries for all known targets.
func AllRegistries() (map[string]*domcap.CapabilityRegistry, error) {
	result := make(map[string]*domcap.CapabilityRegistry, len(registryFileSpec))
	for target := range registryFileSpec {
		r, err := LoadRegistry(target)
		if err != nil {
			return nil, err
		}
		result[target] = r
	}
	return result, nil
}
