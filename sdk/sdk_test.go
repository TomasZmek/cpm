package sdk

import (
	"context"
	"testing"
)

type nopSource struct{}

func (nopSource) Containers(context.Context) ([]Container, error) { return nil, nil }

func TestRegisterAndLookup(t *testing.T) {
	Register(Module{ID: "test-mod", New: func(Config) (Source, error) { return nopSource{}, nil }})
	if _, ok := Lookup("test-mod"); !ok {
		t.Fatal("registered module not found")
	}
	for _, bad := range []Module{
		{ID: "Bad", New: func(Config) (Source, error) { return nil, nil }},
		{ID: "../x", New: func(Config) (Source, error) { return nil, nil }},
		{ID: "no-new"},
		{ID: "test-mod", New: func(Config) (Source, error) { return nil, nil }},
	} {
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("Register(%q) did not panic", bad.ID)
				}
			}()
			Register(bad)
		}()
	}
}

func TestTextFallback(t *testing.T) {
	tx := Text{"en": "Hello", "cs": "Ahoj"}
	if tx.In("cs") != "Ahoj" || tx.In("ko") != "Hello" {
		t.Fatalf("unexpected %q %q", tx.In("cs"), tx.In("ko"))
	}
	if (Text{"de": "Hallo"}).In("cs") != "Hallo" {
		t.Fatal("no fallback to any text")
	}
}
