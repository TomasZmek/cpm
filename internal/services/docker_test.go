package services

import (
	"errors"
	"fmt"
	"sync"
	"syscall"
	"testing"
)

// Run with -race: reconnect() swaps the client while other goroutines use it (#33).
func TestDockerReconnectConcurrent(t *testing.T) {
	t.Setenv("DOCKER_HOST", "unix:///nonexistent/docker.sock")
	d := NewDockerService("caddy")

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			_ = d.IsAvailable()
			_, _ = d.GetContainerID()
		}()
		go func() {
			defer wg.Done()
			d.reconnect(d.cli())
		}()
	}
	wg.Wait()

	if d.cli() == nil {
		t.Fatal("expected a client after reconnect")
	}
}

func TestReconnectSkipsWhenAlreadyReplaced(t *testing.T) {
	t.Setenv("DOCKER_HOST", "unix:///nonexistent/docker.sock")
	d := NewDockerService("caddy")
	stale := d.cli()
	d.reconnect(stale)
	fresh := d.cli()
	if fresh == stale {
		t.Fatal("expected the client to be replaced")
	}
	// A second caller that still holds the stale client must not reconnect again
	d.reconnect(stale)
	if d.cli() != fresh {
		t.Error("reconnect replaced an already refreshed client")
	}
}

func TestIsDockerUnreachable(t *testing.T) {
	cases := map[string]bool{
		"nil":          false,
		"unavailable":  true,
		"econnrefused": true,
		"other":        false,
	}
	errs := map[string]error{
		"nil":          nil,
		"unavailable":  fmt.Errorf("wrap: %w", errDockerUnavailable),
		"econnrefused": fmt.Errorf("dial: %w", syscall.ECONNREFUSED),
		"other":        errors.New("command failed with exit code 1"),
	}
	for name, want := range cases {
		if got := isDockerUnreachable(errs[name]); got != want {
			t.Errorf("%s: got %v, want %v", name, got, want)
		}
	}
}
