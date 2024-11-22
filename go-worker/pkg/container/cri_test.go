package container

import (
	"context"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/cri-api/pkg/apis/runtime/v1"
	remote "k8s.io/cri-client/pkg"
	"k8s.io/cri-client/pkg/fake"
	"testing"
	"time"
)

func TestCRIFake(t *testing.T) {
	endpoint, err := fake.GenerateEndpoint()
	require.NoError(t, err)

	fakeRuntime := fake.NewFakeRemoteRuntime()
	err = fakeRuntime.Start(endpoint)
	assert.NoError(t, err)

	engine, err := newCriEngine(context.Background(), endpoint)
	assert.NoError(t, err)

	ctr, err := fakeRuntime.CreateContainer(context.Background(), &v1.CreateContainerRequest{
		Config: &v1.ContainerConfig{
			Metadata: &v1.ContainerMetadata{
				Name:    "test_container",
				Attempt: 0,
			},
			Image: &v1.ImageSpec{
				Image: "alpine:3.20.3",
			},
			Labels: map[string]string{"foo": "bar"},
			Envs: []*v1.KeyValue{{
				Key:   "test",
				Value: "container",
			}},
			// These won't get set by the fake implementation of cri
			Linux: &v1.LinuxContainerConfig{
				Resources: &v1.LinuxContainerResources{
					CpuQuota:   2,
					CpusetCpus: "1-3",
				},
				SecurityContext: &v1.LinuxContainerSecurityContext{
					Privileged: true,
				},
			},
		},
		PodSandboxId: "test_sandbox",
	})
	assert.NoError(t, err)

	events, err := engine.List(context.Background())
	assert.NoError(t, err)

	expectedEvent := Event{
		Info: Info{Container{
			Type:             typeCri.ToCTValue(),
			ID:               "test_sandbox",
			Name:             "test_container",
			Image:            "alpine:3.20.3",
			ImageDigest:      "alpine:3.20.3",
			User:             "&ContainerUser{Linux:nil,}",
			CPUPeriod:        defaultCpuPeriod,
			CPUQuota:         0,
			CPUShares:        defaultCpuShares,
			CPUSetCPUCount:   0,
			Env:              nil, // TODO
			FullID:           "test_sandbox_test_container_0",
			Labels:           map[string]string{"foo": "bar", "io.kubernetes.sandbox.id": "test_sandbox_test_container_0"},
			PodSandboxID:     "test_sandbox_test_container_0",
			Privileged:       false, // TODO
			PodSandboxLabels: map[string]string{},
			Mounts:           []mount{},
		}},
		IsCreate: true,
	}

	// We don't have this before creation
	found := false
	for _, event := range events {
		if event.FullID == ctr.ContainerId {
			found = true
			// We don't have this before creation
			expectedEvent.CreatedTime = event.CreatedTime
			assert.Equal(t, expectedEvent, event)
		}
	}
	assert.True(t, found)

	// fakeruntime.GetContainerEvents() returns nil. Cannot be tested.
}

func TestCRI(t *testing.T) {
	const crioSocket = "/run/crio/crio.sock"
	client, err := remote.NewRemoteRuntimeService(crioSocket, 5*time.Second, nil, nil)
	if err != nil {
		t.Skip("CRI socket " + crioSocket + " mandatory to run cri tests")
	}

	engine, err := newCriEngine(context.Background(), crioSocket)
	assert.NoError(t, err)

	ctr, err := client.CreateContainer(context.Background(), "test_sandbox", &v1.ContainerConfig{
		Metadata: &v1.ContainerMetadata{
			Name:    "test_container",
			Attempt: 0,
		},
		Image: &v1.ImageSpec{
			Image: "alpine:3.20.3",
		},
		Labels: map[string]string{"foo": "bar"},
		Envs: []*v1.KeyValue{{
			Key:   "test",
			Value: "container",
		}},
		Linux: &v1.LinuxContainerConfig{
			Resources: &v1.LinuxContainerResources{
				CpuQuota:   2000,
				CpusetCpus: "1-3",
			},
			SecurityContext: &v1.LinuxContainerSecurityContext{
				Privileged: true,
			},
		},
	}, nil)
	assert.NoError(t, err)

	events, err := engine.List(context.Background())
	assert.NoError(t, err)

	expectedEvent := Event{
		Info: Info{Container{
			Type:             typeCri.ToCTValue(),
			ID:               "test_sandbox",
			Name:             "test_container",
			Image:            "alpine:3.20.3",
			ImageDigest:      "alpine:3.20.3",
			User:             "&ContainerUser{Linux:nil,}",
			CPUPeriod:        defaultCpuPeriod,
			CPUQuota:         2000,
			CPUShares:        defaultCpuShares,
			CPUSetCPUCount:   3,
			Env:              nil, // TODO
			FullID:           "test_sandbox_test_container_0",
			Labels:           map[string]string{"foo": "bar", "io.kubernetes.sandbox.id": "test_sandbox_test_container_0"},
			PodSandboxID:     "test_sandbox_test_container_0",
			Privileged:       false, // TODO
			PodSandboxLabels: map[string]string{},
			Mounts:           []mount{},
		}},
		IsCreate: true,
	}

	// We don't have this before creation
	found := false
	for _, event := range events {
		if event.FullID == ctr {
			found = true
			// We don't have this before creation
			expectedEvent.CreatedTime = event.CreatedTime
			assert.Equal(t, expectedEvent, event)
		}
	}
	assert.True(t, found)

	// Now try the listen API
	// fakeruntime.GetContainerEvents() returns nil. Cannot be tested rn.
	/*cancelCtx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	listCh, err := engine.Listen(cancelCtx)
	assert.NoError(t, err)

	_, err = fakeRuntime.RemoveContainer(context.Background(), &v1.RemoveContainerRequest{
		ContainerId: "test_sandbox_test_container_0",
	})
	assert.NoError(t, err)

	// receive the "remove" event
	event := <-listCh
	assert.Equal(t, nil, event)*/
}
