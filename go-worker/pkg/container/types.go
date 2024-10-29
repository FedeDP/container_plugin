package container

import (
	"context"
	"fmt"
	"github.com/FedeDP/container-worker/pkg/container/clients"
)

type Type int32

const (
	CtDocker     Type = 0
	CtLxc        Type = 1
	CtLibvirtLxc Type = 2
	CtMesos      Type = 3
	CtRkt        Type = 4 // deprecated
	CtCustom     Type = 5
	CtCri        Type = 6
	CtContainerd Type = 7
	CtCrio       Type = 8
	CtBpm        Type = 9
	CtStatic     Type = 10
	CtPodman     Type = 11

	// Default value, may be changed if necessary
	CtUnknown Type = 0xfff
)

func (ct Type) String() string {
	switch ct {
	case CtDocker:
		return "docker"
	case CtLxc:
		return "lxc"
	case CtLibvirtLxc:
		return "libvirt-lxc"
	case CtMesos:
		return "mesos"
	case CtRkt:
		return "rkt"
	case CtCri:
		return "cri"
	case CtContainerd:
		return "containerd"
	case CtCrio:
		return "cri-o"
	case CtBpm:
		return "bpm"
	case CtPodman:
		return "podman"
	default:
		return "unknown"
	}
}

func (ct Type) ToClient(ctx context.Context) (clients.Client, error) {
	switch ct {
	case CtDocker:
		return clients.NewDockerClient(ctx)
	case CtLxc:
		return nil, fmt.Errorf("unsupported")
	case CtLibvirtLxc:
		return nil, fmt.Errorf("unsupported")
	case CtMesos:
		return nil, fmt.Errorf("unsupported")
	case CtRkt:
		return nil, fmt.Errorf("unsupported")
	case CtCri:
		return nil, fmt.Errorf("unsupported")
	case CtContainerd:
		return nil, fmt.Errorf("unsupported")
	case CtCrio:
		return nil, fmt.Errorf("unsupported")
	case CtBpm:
		return nil, fmt.Errorf("unsupported")
	case CtPodman:
		return clients.NewPodmanClient(ctx)
	default:
		return nil, fmt.Errorf("wrong container type")
	}
}
