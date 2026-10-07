// Package views renders templates from one of several UI themes.
package views

import (
	"fmt"
	"io"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/template/html/v2"
)

// ThemeKey is the view-binding key that selects the theme for a render.
// The theme middleware sets it via c.Bind.
const ThemeKey = "theme"

// Themed implements fiber.Views with one template engine per theme
// (templates/themes/<name>). Each render uses the theme named in the
// binding, falling back to the default theme.
type Themed struct {
	engines map[string]*html.Engine
	def     string
}

// New creates engines for the given themes under root.
func New(root string, themes []string, def string, funcs map[string]interface{}, reload bool) *Themed {
	t := &Themed{engines: make(map[string]*html.Engine), def: def}
	for _, name := range themes {
		engine := html.New(root+"/"+name, ".html")
		for k, fn := range funcs {
			engine.AddFunc(k, fn)
		}
		engine.Reload(reload)
		t.engines[name] = engine
	}
	return t
}

// Load parses the templates of every theme.
func (t *Themed) Load() error {
	for name, engine := range t.engines {
		if err := engine.Load(); err != nil {
			return fmt.Errorf("theme %s: %w", name, err)
		}
	}
	return nil
}

// Render renders a template of the theme selected in the binding.
func (t *Themed) Render(out io.Writer, name string, binding interface{}, layout ...string) error {
	engine := t.engines[t.def]
	var theme interface{}
	switch m := binding.(type) {
	case fiber.Map:
		theme = m[ThemeKey]
	case map[string]interface{}:
		theme = m[ThemeKey]
	}
	if s, ok := theme.(string); ok {
		if e, ok := t.engines[s]; ok {
			engine = e
		}
	}
	return engine.Render(out, name, binding, layout...)
}
