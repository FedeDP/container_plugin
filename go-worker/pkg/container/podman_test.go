package container

import (
	"context"
	"github.com/containers/podman/v5/pkg/bindings"
	"github.com/containers/podman/v5/pkg/bindings/containers"
	"github.com/containers/podman/v5/pkg/specgen"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestPodman(t *testing.T) {
	const userPodmanSocket = "/run/user/1000/podman/podman.sock"
	podmanClient, err := bindings.NewConnection(context.Background(), enforceUnixProtocolIfEmpty(userPodmanSocket))
	if err != nil {
		t.Skip("Podman socket " + userPodmanSocket + " mandatory to run podman tests")
	}

	engine, err := newPodmanEngine(context.Background(), userPodmanSocket)
	assert.NoError(t, err)

	privileged := true
	var cpuQuota int64 = 2000
	ctr, err := containers.CreateWithSpec(podmanClient, &specgen.SpecGenerator{
		ContainerBasicConfig: specgen.ContainerBasicConfig{
			Name:   "test_container",
			Env:    map[string]string{"env": "env"},
			Labels: map[string]string{"foo": "bar"},
		},
		ContainerStorageConfig: specgen.ContainerStorageConfig{
			Image: "alpine:3.20.3",
		},
		ContainerSecurityConfig: specgen.ContainerSecurityConfig{
			Privileged: &privileged,
			User:       "testuser",
		},
		ContainerResourceConfig: specgen.ContainerResourceConfig{
			ResourceLimits: &specs.LinuxResources{
				CPU: &specs.LinuxCPU{
					Quota: &cpuQuota,
					Cpus:  "1-3",
				},
			},
		},
	}, nil)
	assert.NoError(t, err)

	events, err := engine.List(context.Background())
	assert.NoError(t, err)

	expectedEvent := Event{
		Info: Info{Container{
			Type:           typePodman.ToCTValue(),
			ID:             ctr.ID[:shortIDLength],
			Name:           "test_container",
			Image:          "alpine:3.20.3",
			ImageDigest:    "sha256:1e42bbe2508154c9126d48c2b8a75420c3544343bf86fd041fb7527e017a4b4a",
			ImageID:        "63b790fccc9078ab8bb913d94a5d869e19fca9b77712b315da3fa45bb8f14636",
			ImageRepo:      "alpine",
			ImageTag:       "3.20.3",
			User:           "testuser",
			CPUPeriod:      defaultCpuPeriod,
			CPUQuota:       2000,
			CPUShares:      defaultCpuShares,
			CPUSetCPUCount: 3, // 1-3
			Env:            []string{"env=env", "PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"},
			FullID:         ctr.ID,
			Labels:         map[string]string{"foo": "bar"},
			Privileged:     true,
			Mounts:         []mount{},
			PortMappings:   []portMapping{},
		}},
		IsCreate: true,
	}

	found := false
	for _, event := range events {
		if event.FullID == ctr.ID {
			found = true
			// We don't have this before creation
			expectedEvent.CreatedTime = event.CreatedTime
			assert.Equal(t, expectedEvent, event)
		}
	}
	assert.True(t, found)

	cancelCtx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	listCh, err := engine.Listen(cancelCtx)
	assert.NoError(t, err)

	_, err = containers.Remove(context.Background(), ctr.ID, nil)
	assert.NoError(t, err)

	// receive the "remove" event
	expectedEvent = Event{
		Info: Info{Container{
			Type:   typePodman.ToCTValue(),
			ID:     ctr.ID[:shortIDLength],
			FullID: ctr.ID,
			Image:  "alpine:3.20.3",
		}},
		IsCreate: false,
	}

	event := <-listCh
	assert.Equal(t, expectedEvent, event)
}
