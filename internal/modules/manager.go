// Package modules keeps track of which CPM modules are enabled and how they
// are configured. The state lives in caddy-config/modules.json: enabling a
// module writes its entry, disabling it removes the entry again.
package modules

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/TomasZmek/cpm/sdk"
)

// FileName is the configuration file inside the CPM config directory.
const FileName = "modules.json"

// maxValueLen limits a single configuration value.
const maxValueLen = 2048

// Active is an enabled module with its running source.
type Active struct {
	Module sdk.Module
	Config sdk.Config
	Source sdk.Source
	Err    error // error from creating the source
}

// Manager loads, saves and instantiates module configurations.
type Manager struct {
	path string

	mu       sync.RWMutex
	configs  map[string]sdk.Config
	active   map[string]*Active
	onChange []func()
}

// NewManager loads modules.json from configDir. Without the file, modules
// marked DefaultEnabled are active with their default configuration.
//
// On a read or parse error the returned manager is still usable (no modules
// active) together with the error; saving from the settings page then
// replaces the broken file.
func NewManager(configDir string) (*Manager, error) {
	m := &Manager{path: filepath.Join(configDir, FileName), active: map[string]*Active{}, configs: map[string]sdk.Config{}}
	configs, err := m.load()
	if err != nil {
		return m, err
	}
	m.configs = configs
	for id, cfg := range configs {
		if mod, ok := sdk.Lookup(id); ok {
			m.active[id] = instantiate(mod, cfg)
		}
	}
	return m, nil
}

func (m *Manager) load() (map[string]sdk.Config, error) {
	data, err := os.ReadFile(m.path)
	if errors.Is(err, os.ErrNotExist) {
		configs := map[string]sdk.Config{}
		for _, mod := range sdk.Modules() {
			if mod.DefaultEnabled {
				configs[mod.ID] = defaults(mod)
			}
		}
		return configs, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", FileName, err)
	}
	configs := map[string]sdk.Config{}
	if err := json.Unmarshal(data, &configs); err != nil {
		return nil, fmt.Errorf("parse %s: %w", FileName, err)
	}
	for id, cfg := range configs {
		if cfg == nil {
			configs[id] = sdk.Config{}
		}
	}
	return configs, nil
}

// save writes modules.json (0600, it may contain tokens) atomically.
func (m *Manager) save() error {
	data, err := json.MarshalIndent(m.configs, "", "  ")
	if err != nil {
		return err
	}
	tmp := m.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, m.path)
}

func defaults(mod sdk.Module) sdk.Config {
	cfg := sdk.Config{}
	for _, f := range mod.Fields {
		if f.Default != "" {
			cfg[f.Key] = f.Default
		}
	}
	return cfg
}

func instantiate(mod sdk.Module, cfg sdk.Config) *Active {
	a := &Active{Module: mod, Config: cfg}
	a.Source, a.Err = mod.New(cfg)
	if a.Err == nil && a.Source == nil {
		a.Err = errors.New("module returned no source")
	}
	return a
}

// OnChange registers a callback run after a module was enabled, changed or
// disabled (e.g. to refresh the inventory).
func (m *Manager) OnChange(fn func()) {
	m.mu.Lock()
	m.onChange = append(m.onChange, fn)
	m.mu.Unlock()
}

func (m *Manager) changed() {
	m.mu.RLock()
	fns := append([]func(){}, m.onChange...)
	m.mu.RUnlock()
	for _, fn := range fns {
		go fn()
	}
}

// Active returns the enabled modules that have a working source, sorted by ID.
func (m *Manager) Active() []*Active {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]*Active, 0, len(m.active))
	for _, a := range m.active {
		out = append(out, a)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Module.ID < out[j].Module.ID })
	return out
}

// Info describes a registered module for the settings page.
type Info struct {
	Module  sdk.Module
	Enabled bool
	Config  sdk.Config // secrets removed
	Err     error
}

// List returns all registered modules with their state.
func (m *Manager) List() []Info {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var out []Info
	for _, mod := range sdk.Modules() {
		info := Info{Module: mod}
		if cfg, ok := m.configs[mod.ID]; ok {
			info.Enabled = true
			info.Config = publicConfig(mod, cfg)
			if a := m.active[mod.ID]; a != nil {
				info.Err = a.Err
			}
		}
		out = append(out, info)
	}
	return out
}

func publicConfig(mod sdk.Module, cfg sdk.Config) sdk.Config {
	out := sdk.Config{}
	for _, f := range mod.Fields {
		if !f.Secret() {
			out[f.Key] = cfg[f.Key]
		}
	}
	return out
}

// HasValue reports whether an enabled module has a non-empty value for key.
func (m *Manager) HasValue(id, key string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.configs[id][key] != ""
}

// Enable enables (or reconfigures) a module. Values for unknown keys are
// dropped; an empty secret keeps the stored one.
func (m *Manager) Enable(id string, values map[string]string) error {
	mod, ok := sdk.Lookup(id)
	if !ok {
		return fmt.Errorf("unknown module %q", id)
	}

	m.mu.Lock()
	old := m.configs[id]
	cfg := sdk.Config{}
	for _, f := range mod.Fields {
		v := strings.TrimSpace(values[f.Key])
		if v == "" && f.Secret() && old != nil {
			v = old[f.Key]
		}
		if err := validate(f, v); err != nil {
			m.mu.Unlock()
			return err
		}
		if v != "" {
			cfg[f.Key] = v
		}
	}
	a := instantiate(mod, cfg)
	if a.Err != nil {
		m.mu.Unlock()
		return a.Err
	}
	prevConfigs, prevActive := m.configs[id], m.active[id]
	m.configs[id] = cfg
	m.active[id] = a
	if err := m.save(); err != nil {
		// Keep memory and file consistent.
		if prevConfigs == nil {
			delete(m.configs, id)
			delete(m.active, id)
		} else {
			m.configs[id], m.active[id] = prevConfigs, prevActive
		}
		m.mu.Unlock()
		return fmt.Errorf("save %s: %w", FileName, err)
	}
	m.mu.Unlock()
	m.changed()
	return nil
}

// Disable disables a module and removes its configuration from modules.json.
func (m *Manager) Disable(id string) error {
	m.mu.Lock()
	cfg, ok := m.configs[id]
	if !ok {
		m.mu.Unlock()
		return nil
	}
	a := m.active[id]
	delete(m.configs, id)
	delete(m.active, id)
	if err := m.save(); err != nil {
		m.configs[id], m.active[id] = cfg, a
		m.mu.Unlock()
		return fmt.Errorf("save %s: %w", FileName, err)
	}
	m.mu.Unlock()
	m.changed()
	return nil
}

func validate(f sdk.Field, v string) error {
	name := f.Label.In("en")
	if name == "" {
		name = f.Key
	}
	if v == "" {
		if f.Required {
			return fmt.Errorf("%s is required", name)
		}
		return nil
	}
	if len(v) > maxValueLen || strings.ContainsAny(v, "\r\n\x00") {
		return fmt.Errorf("%s: invalid value", name)
	}
	if f.Type == sdk.FieldURL {
		u, err := url.Parse(v)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			return fmt.Errorf("%s: expected an http(s) URL", name)
		}
	}
	return nil
}
