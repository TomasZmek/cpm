package modules

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/TomasZmek/cpm/sdk"
)

type fakeSource struct{ cfg sdk.Config }

func (f *fakeSource) Containers(context.Context) ([]sdk.Container, error) { return nil, nil }

func init() {
	sdk.Register(sdk.Module{
		ID:             "testdefault",
		DefaultEnabled: true,
		New:            func(cfg sdk.Config) (sdk.Source, error) { return &fakeSource{cfg}, nil },
	})
	sdk.Register(sdk.Module{
		ID: "testremote",
		Fields: []sdk.Field{
			{Key: "url", Type: sdk.FieldURL, Required: true},
			{Key: "token", Type: sdk.FieldPassword, Required: true},
		},
		New: func(cfg sdk.Config) (sdk.Source, error) { return &fakeSource{cfg}, nil },
	})
}

func readFile(t *testing.T, dir string) map[string]sdk.Config {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, FileName))
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]sdk.Config
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

func activeIDs(m *Manager) map[string]bool {
	ids := map[string]bool{}
	for _, a := range m.Active() {
		ids[a.Module.ID] = true
	}
	return ids
}

func TestDefaultsWithoutFile(t *testing.T) {
	m, err := NewManager(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	ids := activeIDs(m)
	if !ids["testdefault"] || ids["testremote"] {
		t.Fatalf("active = %v", ids)
	}
}

func TestEnableWritesAndDisableRemoves(t *testing.T) {
	dir := t.TempDir()
	m, _ := NewManager(dir)

	if err := m.Enable("testremote", map[string]string{"url": "http://dockhand:3000", "token": "dh_secret"}); err != nil {
		t.Fatal(err)
	}
	file := readFile(t, dir)
	if file["testremote"]["token"] != "dh_secret" || file["testdefault"] == nil {
		t.Fatalf("modules.json = %v", file)
	}
	if fi, _ := os.Stat(filepath.Join(dir, FileName)); fi.Mode().Perm() != 0o600 {
		t.Fatalf("modules.json mode = %v", fi.Mode().Perm())
	}

	// Empty secret keeps the stored one; secrets are not exposed by List.
	if err := m.Enable("testremote", map[string]string{"url": "http://dockhand:3001"}); err != nil {
		t.Fatal(err)
	}
	if readFile(t, dir)["testremote"]["token"] != "dh_secret" {
		t.Fatal("secret was not kept")
	}
	for _, info := range m.List() {
		if _, leaked := info.Config["token"]; leaked {
			t.Fatal("secret exposed in List")
		}
	}

	if err := m.Disable("testremote"); err != nil {
		t.Fatal(err)
	}
	if _, ok := readFile(t, dir)["testremote"]; ok {
		t.Fatal("disabled module still in modules.json")
	}

	// Disabling a default module persists across restarts.
	m.Disable("testdefault")
	m2, _ := NewManager(dir)
	if ids := activeIDs(m2); ids["testdefault"] || ids["testremote"] {
		t.Fatalf("after restart active = %v", ids)
	}
}

func TestEnableValidates(t *testing.T) {
	m, _ := NewManager(t.TempDir())
	for _, values := range []map[string]string{
		{"token": "x"},                               // url missing
		{"url": "ftp://host", "token": "x"},          // wrong scheme
		{"url": "javascript:alert(1)", "token": "x"}, // no host
		{"url": "http://host", "token": "a\nb"},      // newline
		{"url": "http://host"},                       // secret missing
	} {
		if err := m.Enable("testremote", values); err == nil {
			t.Errorf("Enable(%v) accepted", values)
		}
	}
	if err := m.Enable("nope", nil); err == nil {
		t.Error("unknown module accepted")
	}
}

func TestCorruptFileStillUsable(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, FileName), []byte("{broken"), 0o600)
	m, err := NewManager(dir)
	if err == nil || m == nil {
		t.Fatalf("NewManager = %v, %v", m, err)
	}
	if len(m.Active()) != 0 {
		t.Fatal("modules active from a broken file")
	}
}
