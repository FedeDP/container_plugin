package container

import (
	"context"
	containerd "github.com/containerd/containerd/v2/client"
	"github.com/containerd/containerd/v2/pkg/namespaces"
	"github.com/containerd/containerd/v2/pkg/oci"
	"github.com/google/uuid"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/stretchr/testify/assert"
	"os/user"
	"sync"
	"testing"
)

func TestContainerd(t *testing.T) {
	usr, err := user.Current()
	assert.NoError(t, err)
	if usr.Uid != "0" {
		t.Skip("Containerd test requires root user")
	}

	const containerdSocket = "/run/containerd/containerd.sock"
	client, err := containerd.New(containerdSocket)
	if err != nil {
		t.Skip("Socket "+containerdSocket+" mandatory to run containerd tests:", err.Error())
	}

	engine, err := newContainerdEngine(context.Background(), containerdSocket)
	assert.NoError(t, err)

	namespacedCtx := namespaces.WithNamespace(context.Background(), "test_ns")

	// Pull image
	if _, err = client.GetImage(namespacedCtx, "docker.io/library/alpine:3.20.3"); err != nil {
		_, err = client.Pull(namespacedCtx, "docker.io/library/alpine:3.20.3")
		assert.NoError(t, err)
	}

	id := uuid.New()
	var cpuQuota int64 = 2000
	ctr, err := client.NewContainer(namespacedCtx, id.String(), containerd.WithImageName("docker.io/library/alpine:3.20.3"),
		containerd.WithSpec(&oci.Spec{
			Process: &specs.Process{
				User: specs.User{
					UID:            0,
					GID:            0,
					Umask:          nil,
					AdditionalGids: nil,
					Username:       "testuser",
				},
			},
			Linux: &specs.Linux{
				Resources: &specs.LinuxResources{
					CPU: &specs.LinuxCPU{
						Quota: &cpuQuota,
						Cpus:  "0-1",
					},
				},
			},
		}))
	assert.NoError(t, err)

	events, err := engine.List(context.Background())
	assert.NoError(t, err)

	expectedEvent := Event{
		Info: Info{Container{
			Type:             typeContainerd.ToCTValue(),
			ID:               ctr.ID()[:shortIDLength],
			Image:            "docker.io/library/alpine:3.20.3",
			CPUPeriod:        defaultCpuPeriod,
			CPUQuota:         cpuQuota,
			CPUShares:        defaultCpuShares,
			CPUSetCPUCount:   2,   // 0-1
			Env:              nil, // TODO
			FullID:           ctr.ID(),
			Labels:           map[string]string{},
			PodSandboxID:     "",
			Privileged:       false, // TODO
			PodSandboxLabels: nil,
			Mounts:           []mount{},
			User:             "testuser",
		}},
		IsCreate: true,
	}

	found := false
	for _, event := range events {
		if event.FullID == ctr.ID() {
			found = true
			// We don't have these before creation
			expectedEvent.CreatedTime = event.CreatedTime
			expectedEvent.Ip = event.Ip
			assert.Equal(t, expectedEvent, event)
		}
	}
	assert.True(t, found)

	// Now try the listen API
	wg := sync.WaitGroup{}
	cancelCtx, cancel := context.WithCancel(context.Background())
	t.Cleanup(func() {
		cancel()
		wg.Wait()
	})

	listCh, err := engine.Listen(cancelCtx, &wg)
	assert.NoError(t, err)

	err = ctr.Delete(namespacedCtx)
	assert.NoError(t, err)

	expectedEvent = Event{
		Info: Info{Container{
			Type:   typeContainerd.ToCTValue(),
			ID:     ctr.ID()[:shortIDLength],
			FullID: ctr.ID(),
		}},
		IsCreate: false,
	}

	// receive the "remove" event
	event := waitOnChannelOrTimeout(t, listCh)
	assert.Equal(t, expectedEvent, event)
}
