// Package dockerlocal is the built-in module that reads containers from the
// Docker daemon CPM is connected to (the mounted Docker socket).
package dockerlocal

import (
	"context"
	"fmt"
	"net"

	"github.com/TomasZmek/cpm/internal/services"
	"github.com/TomasZmek/cpm/sdk"
)

// ID of the module in modules.json.
const ID = "docker"

// Register registers the module. defaultHostIP returns the IP of the host
// marked as local in the Docker discovery hosts, used when host_ip is empty.
func Register(docker *services.DockerService, defaultHostIP func() (ip, label string)) {
	sdk.Register(sdk.Module{
		ID:   ID,
		Icon: "🐳",
		Name: sdk.Text{"en": "Docker (local)", "cs": "Docker (lokální)", "ko": "Docker (로컬)"},
		Description: sdk.Text{
			"en": "Containers of the Docker host CPM runs on (mounted Docker socket).",
			"cs": "Kontejnery Docker hosta, na kterém běží CPM (připojený Docker socket).",
			"ko": "CPM이 실행되는 Docker 호스트의 컨테이너 (마운트된 Docker 소켓).",
		},
		DefaultEnabled: true,
		Fields: []sdk.Field{{
			Key:         "host_ip",
			Type:        sdk.FieldText,
			Placeholder: "192.168.1.10",
			Label:       sdk.Text{"en": "Docker host IP", "cs": "IP adresa Docker hosta", "ko": "Docker 호스트 IP"},
			Help: sdk.Text{
				"en": "Address of this host as used in proxy rules. Empty: the host marked as local in Settings → Docker.",
				"cs": "Adresa tohoto hosta, jak ji používají proxy pravidla. Prázdné: host označený jako lokální v Nastavení → Docker.",
				"ko": "프록시 규칙에서 사용하는 이 호스트의 주소. 비어 있으면 설정 → Docker에서 로컬로 표시된 호스트.",
			},
		}},
		New: func(cfg sdk.Config) (sdk.Source, error) {
			ip := cfg["host_ip"]
			if ip != "" && net.ParseIP(ip) == nil {
				return nil, fmt.Errorf("invalid Docker host IP %q", ip)
			}
			return &source{docker: docker, hostIP: ip, defaultHostIP: defaultHostIP}, nil
		},
	})
}

type source struct {
	docker        *services.DockerService
	hostIP        string
	defaultHostIP func() (string, string)
}

func (s *source) Containers(ctx context.Context) ([]sdk.Container, error) {
	ip, label := s.hostIP, ""
	if def, defLabel := s.defaultHostIP(); ip == "" || ip == def {
		if ip == "" {
			ip = def
		}
		label = defLabel
	}
	return s.docker.ListContainers(ctx, ip, label)
}
