package inventory

import (
	"context"
	"net"
	"strconv"
	"testing"

	"github.com/TomasZmek/cpm/internal/models"
	"github.com/TomasZmek/cpm/internal/modules"
	"github.com/TomasZmek/cpm/sdk"
)

var testContainers = []sdk.Container{
	{Name: "anki-sync", State: "running", HostIP: "192.168.50.222", Host: "NAS", Ports: []sdk.Port{{Private: 8080, Public: 8080}}},
	{Name: "forgejo", State: "running", HostIP: "192.168.50.222", Ports: []sdk.Port{{Private: 3000, Public: 3053}}},
	{Name: "jellyfin", State: "exited", HostIP: "192.168.50.159"},
	{Name: "other", State: "running", HostIP: "192.168.50.159", Ports: []sdk.Port{{Private: 8080, Public: 8080}}},
}

type staticSource struct{}

func (staticSource) Containers(context.Context) ([]sdk.Container, error) { return testContainers, nil }

func init() {
	sdk.Register(sdk.Module{ID: "invtest", DefaultEnabled: true, New: func(sdk.Config) (sdk.Source, error) { return staticSource{}, nil }})
}

func site(ip, port string) *models.Site { return &models.Site{TargetIP: ip, TargetPort: port} }

func TestMatch(t *testing.T) {
	cases := []struct {
		site *models.Site
		want string
	}{
		{site("192.168.50.222", "8080"), "anki-sync"},
		{site("192.168.50.159", "8080"), "other"}, // same port, other host
		{site("192.168.50.222", "3053"), "forgejo"},
		{site("192.168.50.222", "3000"), ""}, // inner port is not reachable on the host IP
		{site("jellyfin", "8096"), "jellyfin"},
		{site("Anki-Sync", "8080"), "anki-sync"},
		{site("192.168.50.10", "8080"), ""},
	}
	for _, c := range cases {
		got := match(c.site, testContainers)
		name := ""
		if got != nil {
			name = got.Name
		}
		if name != c.want {
			t.Errorf("match(%s:%s) = %q, want %q", c.site.TargetIP, c.site.TargetPort, name, c.want)
		}
	}
}

func TestStatusAndProbe(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	openPort := strconv.Itoa(ln.Addr().(*net.TCPAddr).Port)
	closed, _ := net.Listen("tcp", "127.0.0.1:0")
	closedPort := strconv.Itoa(closed.Addr().(*net.TCPAddr).Port)
	closed.Close()

	sites := []*models.Site{
		site("192.168.50.222", "8080"),
		site("jellyfin", "8096"),
		site("127.0.0.1", openPort),
		site("127.0.0.1", closedPort),
		site("someservice", "80"), // single-label name: not probed
	}
	mods, _ := modules.NewManager(t.TempDir())
	inv := New(mods, func() []*models.Site { return sites })
	inv.Refresh(context.Background())

	want := []Status{
		{State: Running, Container: "anki-sync", Host: "NAS"},
		{State: Stopped, Container: "jellyfin"},
		{State: Reachable},
		{State: Unreachable},
		{State: Unknown},
	}
	for i, s := range sites {
		if got := inv.StatusFor(s); got != want[i] {
			t.Errorf("StatusFor(%s:%s) = %+v, want %+v", s.TargetIP, s.TargetPort, got, want[i])
		}
	}
	if srcs := inv.Sources(); len(srcs) == 0 || srcs[0].Count != len(testContainers) {
		t.Fatalf("sources = %+v", srcs)
	}
}

type dupSource struct{}

func (dupSource) Containers(context.Context) ([]sdk.Container, error) {
	return []sdk.Container{
		{ID: "0123456789abcdef", Name: "anki-sync", State: "running", HostIP: "192.168.50.222"},
		{ID: "0123456789ab", Name: "anki-sync", State: "running", HostIP: "192.168.50.222"},
		{Name: "noid", State: "running"},
		{Name: "noid", State: "running"},
	}, nil
}

func TestDuplicateContainersMerged(t *testing.T) {
	sdk.Register(sdk.Module{ID: "invdup", New: func(sdk.Config) (sdk.Source, error) { return dupSource{}, nil }})
	dir := t.TempDir()
	mods, _ := modules.NewManager(dir)
	mods.Disable("invtest")
	if err := mods.Enable("invdup", nil); err != nil {
		t.Fatal(err)
	}
	inv := New(mods, func() []*models.Site { return nil })
	inv.Refresh(context.Background())
	if n := len(inv.Containers()); n != 3 {
		t.Fatalf("got %d containers, want 3 (same ID merged, no-ID kept)", n)
	}
}
