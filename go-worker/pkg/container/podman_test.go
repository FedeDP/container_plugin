package container

import (
	"context"
	"fmt"
	"github.com/containers/podman/v5/pkg/bindings"
	"github.com/containers/podman/v5/pkg/bindings/containers"
	"github.com/containers/podman/v5/pkg/bindings/images"
	"github.com/containers/podman/v5/pkg/specgen"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/stretchr/testify/assert"
	"os/user"
	"testing"
)

func TestPodman(t *testing.T) {
	usr, err := user.Current()
	assert.NoError(t, err)

	var podmanSocket string
	if usr.Name != "root" {
		podmanSocket = "/run/user/" + usr.Uid + "/podman/podman.sock"
	} else {
		podmanSocket = fmt.Sprintf("/run/podman/podman.sock")
	}

	podmanCtx, err := bindings.NewConnection(context.Background(), enforceUnixProtocolIfEmpty(podmanSocket))
	if err != nil {
		t.Skip("Socket "+podmanSocket+" mandatory to run podman tests:", err.Error())
	}

	if _, err = images.GetImage(podmanCtx, "alpine:3.20.3", nil); err != nil {
		_, err = images.Pull(podmanCtx, "alpine:3.20.3", nil)
		assert.NoError(t, err)
	}

	engine, err := newPodmanEngine(context.Background(), podmanSocket)
	assert.NoError(t, err)

	privileged := true
	var cpuQuota int64 = 2000
	ctr, err := containers.CreateWithSpec(podmanCtx, &specgen.SpecGenerator{
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
					Cpus:  "0-1",
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
			Image:          "docker.io/library/alpine:3.20.3",
			ImageDigest:    "sha256:1e42bbe2508154c9126d48c2b8a75420c3544343bf86fd041fb7527e017a4b4a",
			ImageID:        "63b790fccc9078ab8bb913d94a5d869e19fca9b77712b315da3fa45bb8f14636",
			ImageRepo:      "docker.io/library/alpine",
			ImageTag:       "3.20.3",
			User:           "testuser",
			CPUPeriod:      defaultCpuPeriod,
			CPUQuota:       2000,
			CPUShares:      defaultCpuShares,
			CPUSetCPUCount: 2, // 0-1
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
			assert.Contains(t, event.Env, "env=env")
			expectedEvent.Env = event.Env
			assert.Equal(t, expectedEvent, event)
		}
	}
	assert.True(t, found)

	cancelCtx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	listCh, err := engine.Listen(cancelCtx)
	assert.NoError(t, err)

	_, err = containers.Remove(podmanCtx, ctr.ID, nil)
	assert.NoError(t, err)

	// receive the "remove" event
	expectedEvent = Event{
		Info: Info{Container{
			Type:   typePodman.ToCTValue(),
			ID:     ctr.ID[:shortIDLength],
			FullID: ctr.ID,
			Image:  "docker.io/library/alpine:3.20.3",
		}},
		IsCreate: false,
	}
	event := <-listCh
	assert.Equal(t, expectedEvent, event)
}
