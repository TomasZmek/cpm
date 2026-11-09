package services

import (
	"testing"

	"github.com/TomasZmek/cpm/internal/models"
	"github.com/TomasZmek/cpm/sdk"
)

func TestDiscoverKeepsHostsApart(t *testing.T) {
	containers := []sdk.Container{
		{Name: "anki-sync", State: "running", HostIP: "192.168.50.222", Ports: []sdk.Port{{Private: 8080, Public: 8080}}},
		{Name: "forgejo", State: "running", HostIP: "192.168.50.222", Ports: []sdk.Port{{Private: 3000, Public: 3053}, {Private: 22, Public: 2222}}},
		{Name: "internal", State: "running", HostIP: "192.168.50.222", Ports: []sdk.Port{{Private: 5432}}},
		{Name: "stopped", State: "exited", HostIP: "192.168.50.222"},
		{Name: "caddy_proxy", State: "running", HostIP: "192.168.50.222"},
		{Name: "app", State: "running", HostIP: "192.168.50.159", Ports: []sdk.Port{{Private: 80, Public: 8080}}},
	}
	sites := []*models.Site{
		{Domains: []string{"anki.example.com"}, TargetIP: "192.168.50.222", TargetPort: "8080"},
		{Domains: []string{"git.example.com"}, TargetIP: "192.168.50.222", TargetPort: "3000"}, // inner port: not a pair
	}
	hosts := []models.DiscoveryHost{{IP: "192.168.50.222", Label: "NAS"}}
	skip := func(c sdk.Container) bool { return c.Name == "caddy_proxy" }

	got := Discover(containers, sites, hosts, skip)
	type row struct {
		ip, name string
		port     uint16
		paired   bool
	}
	want := []row{
		{"192.168.50.159", "app", 8080, false},
		{"192.168.50.222", "anki-sync", 8080, true},
		{"192.168.50.222", "forgejo", 3053, false},
		{"192.168.50.222", "forgejo", 2222, false},
		{"192.168.50.222", "internal", 0, false},
	}
	if len(got) != len(want) {
		t.Fatalf("got %d rows: %+v", len(got), got)
	}
	for i, w := range want {
		g := got[i]
		if g.DiscoveryIP != w.ip || g.ContainerName != w.name || g.Port != w.port || g.Paired != w.paired {
			t.Errorf("row %d = %+v, want %+v", i, g, w)
		}
	}
	if got[1].HostLabel != "NAS" {
		t.Errorf("host label = %q", got[1].HostLabel)
	}
}
