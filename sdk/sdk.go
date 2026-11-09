// Package sdk is the public interface between CPM and its modules.
//
// A module is compiled into the CPM binary and registers itself from an
// init function:
//
//	func init() {
//		sdk.Register(sdk.Module{
//			ID:   "example",
//			Name: sdk.Text{"en": "Example", "cs": "Příklad"},
//			Fields: []sdk.Field{
//				{Key: "url", Label: sdk.Text{"en": "URL"}, Type: sdk.FieldURL, Required: true},
//			},
//			New: func(cfg sdk.Config) (sdk.Source, error) { return &source{url: cfg["url"]}, nil },
//		})
//	}
//
// Registered modules appear in Settings → Modules, where an administrator
// enables and configures them at runtime. The configuration is stored in
// caddy-config/modules.json; disabling a module removes its entry.
//
// The package has no dependencies, so modules can be maintained and versioned
// independently of CPM.
package sdk

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"sync"
)

// APIVersion is incremented on incompatible changes to this package.
const APIVersion = 1

// Text is a translated string: language code → text. "en" is the fallback.
type Text map[string]string

// In returns the text for lang, falling back to English, then to any text.
func (t Text) In(lang string) string {
	if s, ok := t[lang]; ok && s != "" {
		return s
	}
	if s, ok := t["en"]; ok && s != "" {
		return s
	}
	keys := make([]string, 0, len(t))
	for k := range t {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if t[k] != "" {
			return t[k]
		}
	}
	return ""
}

// FieldType controls how a configuration field is rendered and validated.
type FieldType string

const (
	FieldText     FieldType = "text"
	FieldURL      FieldType = "url"
	FieldPassword FieldType = "password" // secret: never shown again after saving
)

// Field describes one configuration value of a module.
type Field struct {
	Key         string
	Label       Text
	Help        Text
	Type        FieldType
	Required    bool
	Placeholder string
	Default     string
}

// Secret reports whether the field value must not be shown in the UI.
func (f Field) Secret() bool { return f.Type == FieldPassword }

// Config holds a module's configuration: field key → value.
type Config map[string]string

// Port is a container port. Public is the port published on the Docker
// host (0 when not published).
type Port struct {
	Private uint16
	Public  uint16
}

// Container is a container as reported by a source.
type Container struct {
	ID     string
	Name   string
	State  string // Docker state: running, exited, paused, …
	HostIP string // IP of the Docker host, as used in proxy rules
	Host   string // optional display name of the Docker host
	Ports  []Port
}

// Running reports whether the container is running.
func (c Container) Running() bool { return c.State == "running" }

// Source lists containers. Containers is called periodically from a
// background goroutine and must honour ctx.
type Source interface {
	Containers(ctx context.Context) ([]Container, error)
}

// Module describes a module.
type Module struct {
	ID          string // lowercase identifier, also the key in modules.json
	Name        Text
	Description Text
	Icon        string // emoji
	Fields      []Field
	// DefaultEnabled modules are active when modules.json does not exist yet.
	DefaultEnabled bool
	// New creates a source from a validated configuration.
	New func(cfg Config) (Source, error)
}

var (
	mu       sync.RWMutex
	registry = map[string]Module{}
	idRe     = regexp.MustCompile(`^[a-z][a-z0-9_-]{1,31}$`)
)

// Register adds a module. It panics on an invalid or duplicate ID, which
// surfaces programming errors at startup.
func Register(m Module) {
	if !idRe.MatchString(m.ID) {
		panic(fmt.Sprintf("sdk: invalid module id %q", m.ID))
	}
	if m.New == nil {
		panic(fmt.Sprintf("sdk: module %q has no New function", m.ID))
	}
	mu.Lock()
	defer mu.Unlock()
	if _, dup := registry[m.ID]; dup {
		panic(fmt.Sprintf("sdk: module %q registered twice", m.ID))
	}
	registry[m.ID] = m
}

// Lookup returns a registered module.
func Lookup(id string) (Module, bool) {
	mu.RLock()
	defer mu.RUnlock()
	m, ok := registry[id]
	return m, ok
}

// Modules returns all registered modules sorted by ID.
func Modules() []Module {
	mu.RLock()
	defer mu.RUnlock()
	out := make([]Module, 0, len(registry))
	for _, m := range registry {
		out = append(out, m)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}
