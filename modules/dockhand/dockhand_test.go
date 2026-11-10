package dockhand

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/TomasZmek/cpm/sdk"
)

func fakeDockhand(t *testing.T, token string, calls *atomic.Int32) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if token != "" && r.Header.Get("Authorization") != "Bearer "+token {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/api/environments":
			w.Write([]byte(`[
				{"id":1,"name":"nas","connectionType":"socket","host":null,"publicIp":"192.168.50.222"},
				{"id":2,"name":"pi","connectionType":"hawser-standard","host":"192.168.50.159","publicIp":null},
				{"id":3,"name":"edge","connectionType":"hawser-edge","host":null,"publicIp":null}
			]`))
		case r.URL.Path == "/api/containers" && r.URL.Query().Get("all") == "true":
			switch r.URL.Query().Get("env") {
			case "1":
				w.Write([]byte(`[{"id":"aaa","name":"anki-sync","state":"running","ports":[
					{"IP":"0.0.0.0","PrivatePort":8080,"PublicPort":8080,"Type":"tcp"},
					{"IP":"::","PrivatePort":8080,"PublicPort":8080,"Type":"tcp"},
					{"IP":"127.0.0.1","PrivatePort":9000,"PublicPort":9000,"Type":"tcp"}]},
					{"id":"bbb","name":"db","state":"exited","ports":[]}]`))
			case "2":
				w.Write([]byte(`[{"id":"ccc","name":"app","state":"running","ports":[{"IP":"0.0.0.0","PrivatePort":80,"PublicPort":8080,"Type":"tcp"}]}]`))
			case "3":
				w.WriteHeader(http.StatusInternalServerError)
			}
		default:
			http.NotFound(w, r)
		}
	}))
}

func TestContainers(t *testing.T) {
	var calls atomic.Int32
	srv := fakeDockhand(t, "dh_secret", &calls)
	defer srv.Close()

	src, err := newSource(sdk.Config{"url": srv.URL, "token": "dh_secret", "host_ips": "3=10.0.0.3"})
	if err != nil {
		t.Fatal(err)
	}
	got, err := src.Containers(context.Background())
	if err == nil || !strings.Contains(err.Error(), "edge") {
		t.Errorf("expected an error for environment edge, got %v", err)
	}
	want := map[string]sdk.Container{
		"anki-sync": {HostIP: "192.168.50.222", Host: "nas", State: "running", Ports: []sdk.Port{{Private: 8080, Public: 8080}}},
		"db":        {HostIP: "192.168.50.222", Host: "nas", State: "exited"},
		"app":       {HostIP: "192.168.50.159", Host: "pi", State: "running", Ports: []sdk.Port{{Private: 80, Public: 8080}}},
	}
	if len(got) != len(want) {
		t.Fatalf("got %d containers: %+v", len(got), got)
	}
	for _, c := range got {
		w := want[c.Name]
		if c.HostIP != w.HostIP || c.Host != w.Host || c.State != w.State || len(c.Ports) != len(w.Ports) {
			t.Errorf("%s = %+v, want %+v", c.Name, c, w)
		}
		for i := range w.Ports {
			if c.Ports[i] != w.Ports[i] {
				t.Errorf("%s ports = %v, want %v", c.Name, c.Ports, w.Ports)
			}
		}
	}
}

func TestEnvironmentFilterAndHostMapping(t *testing.T) {
	var calls atomic.Int32
	srv := fakeDockhand(t, "", &calls)
	defer srv.Close()
	src, _ := newSource(sdk.Config{"url": srv.URL + "/", "environments": "2", "host_ips": "2=192.168.50.200"})
	got, err := src.Containers(context.Background())
	if err != nil || len(got) != 1 || got[0].Name != "app" || got[0].HostIP != "192.168.50.200" {
		t.Fatalf("got %+v, %v", got, err)
	}
}

func TestRejectedTokenBacksOff(t *testing.T) {
	var calls atomic.Int32
	srv := fakeDockhand(t, "dh_right", &calls)
	defer srv.Close()
	src, _ := newSource(sdk.Config{"url": srv.URL, "token": "dh_wrong"})
	if _, err := src.Containers(context.Background()); err == nil {
		t.Fatal("wrong token accepted")
	}
	before := calls.Load()
	for i := 0; i < 5; i++ {
		src.Containers(context.Background())
	}
	if calls.Load() != before {
		t.Fatalf("kept calling Dockhand after a rejected token (%d extra calls)", calls.Load()-before)
	}
}

func TestConfigValidation(t *testing.T) {
	for _, cfg := range []sdk.Config{
		{"url": "ftp://x"},
		{"url": "http://x", "token": "abc"},
		{"url": "http://x", "host_ips": "1=notanip"},
		{"url": "http://x", "host_ips": "x=10.0.0.1"},
		{"url": "http://x", "environments": "a,b"},
	} {
		if _, err := newSource(cfg); err == nil {
			t.Errorf("newSource(%v) accepted", cfg)
		}
	}
	if _, err := newSource(sdk.Config{"url": "http://dockhand:3000", "host_ips": "1=192.168.1.10; 2=192.168.1.20", "environments": "1, 2"}); err != nil {
		t.Fatal(err)
	}
}
