package handlers

import (
	"context"
	"strings"
	"time"

	"github.com/TomasZmek/cpm/internal/services"
	"github.com/TomasZmek/cpm/sdk"
	"github.com/gofiber/fiber/v3"
)

// moduleFieldView is one configuration field as rendered by the settings page.
type moduleFieldView struct {
	Key         string
	Label       string
	Help        string
	InputType   string
	Required    bool
	Placeholder string
	Value       string // empty for secrets
	Secret      bool
	HasSecret   bool   // a secret value is stored
	Suggest     string // suggested value (host IP fields)
}

// moduleView is one module as rendered by Settings → Modules.
type moduleView struct {
	ID          string
	Icon        string
	Name        string
	Description string
	Enabled     bool
	Error       string // creating the source failed
	SourceError string // last refresh failed
	Containers  int
	Refreshed   bool
	Fields      []moduleFieldView
}

// SettingsModules renders Settings → Modules.
func (h *Handler) SettingsModules(c fiber.Ctx) error {
	return h.renderSettingsTab(c, "modules")
}

func (h *Handler) moduleViews(c fiber.Ctx) []moduleView {
	if h.modules == nil {
		return nil
	}
	lang := getLang(c)
	hostIP := requestHostIP(c)
	if ip, err := services.DetectLocalIP(); err == nil {
		hostIP = ip
	}
	type srcState struct {
		count int
		err   string
	}
	states := map[string]srcState{}
	refreshed := false
	if h.inventory != nil {
		refreshed = !h.inventory.Updated().IsZero()
		for _, st := range h.inventory.Sources() {
			states[st.ModuleID] = srcState{st.Count, st.Err}
		}
	}

	var out []moduleView
	for _, info := range h.modules.List() {
		mod := info.Module
		v := moduleView{
			ID:          mod.ID,
			Icon:        mod.Icon,
			Name:        mod.Name.In(lang),
			Description: mod.Description.In(lang),
			Enabled:     info.Enabled,
		}
		if v.Icon == "" {
			v.Icon = "🧩"
		}
		if info.Err != nil {
			v.Error = info.Err.Error()
		}
		if st, ok := states[mod.ID]; ok && info.Enabled {
			v.Containers, v.SourceError, v.Refreshed = st.count, st.err, refreshed
		}
		for _, f := range mod.Fields {
			fv := moduleFieldView{
				Key:         f.Key,
				Label:       f.Label.In(lang),
				Help:        f.Help.In(lang),
				InputType:   "text",
				Required:    f.Required,
				Placeholder: f.Placeholder,
				Secret:      f.Secret(),
			}
			switch f.Type {
			case sdk.FieldURL:
				fv.InputType = "url"
			case sdk.FieldPassword:
				fv.InputType = "password"
			}
			if fv.Label == "" {
				fv.Label = f.Key
			}
			if f.Type == sdk.FieldHostIP && hostIP != "" && hostIP != info.Config[f.Key] {
				fv.Suggest = hostIP
			}
			if info.Enabled {
				if fv.Secret {
					fv.HasSecret = h.modules.HasValue(mod.ID, f.Key)
				} else {
					fv.Value = info.Config[f.Key]
				}
			} else if !fv.Secret {
				fv.Value = f.Default
			}
			// A stored secret need not be re-entered.
			if fv.HasSecret {
				fv.Required = false
			}
			v.Fields = append(v.Fields, fv)
		}
		out = append(out, v)
	}
	return out
}

// ModuleSave enables a module or saves its configuration. Form fields are
// named f_<key>.
func (h *Handler) ModuleSave(c fiber.Ctx) error {
	id := c.Params("id")
	mod, ok := sdk.Lookup(id)
	if !ok || h.modules == nil {
		return fiber.NewError(fiber.StatusNotFound, "module not found")
	}
	wasEnabled := false
	for _, info := range h.modules.List() {
		if info.Module.ID == id {
			wasEnabled = info.Enabled
		}
	}

	values := map[string]string{}
	for _, f := range mod.Fields {
		values[f.Key] = c.FormValue("f_" + f.Key)
	}
	name := mod.Name.In(getLang(c))
	if err := h.modules.Enable(id, values); err != nil {
		setFlash(c, "error", tl(c, "msg_module_error", name)+": "+err.Error())
		return redirectTo(c, "/settings/modules")
	}

	// Refresh now so the page shows whether the source works.
	if h.inventory != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		h.inventory.Refresh(ctx)
		cancel()
		for _, st := range h.inventory.Sources() {
			if st.ModuleID == id && st.Err != "" {
				setFlash(c, "warning", tl(c, "msg_module_saved_error", name)+": "+st.Err)
				return redirectTo(c, "/settings/modules")
			}
		}
	}

	key := "msg_module_saved"
	if !wasEnabled {
		key = "msg_module_enabled"
	}
	setFlash(c, "success", tl(c, key, name))
	return redirectTo(c, "/settings/modules")
}

// ModuleDisable disables a module and removes its configuration.
func (h *Handler) ModuleDisable(c fiber.Ctx) error {
	id := c.Params("id")
	mod, ok := sdk.Lookup(id)
	if !ok || h.modules == nil {
		return fiber.NewError(fiber.StatusNotFound, "module not found")
	}
	name := mod.Name.In(getLang(c))
	if err := h.modules.Disable(id); err != nil {
		setFlash(c, "error", tl(c, "msg_module_error", name)+": "+err.Error())
		return redirectTo(c, "/settings/modules")
	}
	if h.inventory != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		h.inventory.Refresh(ctx)
		cancel()
	}
	setFlash(c, "success", tl(c, "msg_module_disabled", strings.TrimSpace(name)))
	return redirectTo(c, "/settings/modules")
}
