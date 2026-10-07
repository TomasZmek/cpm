package views

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/gofiber/fiber/v2"
)

func TestThemedRendersSelectedTheme(t *testing.T) {
	root := t.TempDir()
	for _, theme := range []string{"classic", "modern"} {
		dir := filepath.Join(root, theme, "pages")
		os.MkdirAll(dir, 0755)
		os.WriteFile(filepath.Join(dir, "x.html"), []byte(theme+":{{.Name}}"), 0644)
	}
	v := New(root, []string{"classic", "modern"}, "classic", nil, false)
	if err := v.Load(); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		binding interface{}
		want    string
	}{
		{fiber.Map{"Name": "a"}, "classic:a"},
		{fiber.Map{"Name": "b", ThemeKey: "modern"}, "modern:b"},
		{map[string]interface{}{"Name": "c", ThemeKey: "modern"}, "modern:c"},
		{fiber.Map{"Name": "d", ThemeKey: "unknown"}, "classic:d"},
	}
	for _, tc := range cases {
		var buf bytes.Buffer
		if err := v.Render(&buf, "pages/x", tc.binding); err != nil {
			t.Fatal(err)
		}
		if buf.String() != tc.want {
			t.Errorf("got %q, want %q", buf.String(), tc.want)
		}
	}
}
