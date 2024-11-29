package container

import (
	"context"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/cri-api/pkg/apis/runtime/v1"
	remote "k8s.io/cri-client/pkg"
	"k8s.io/cri-client/pkg/fake"
	"sync"
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
	const criSocket = "/run/containerd/containerd.sock"
	client, err := remote.NewRemoteRuntimeService(criSocket, 5*time.Second, nil, nil)
	if err != nil {
		t.Skip("Socket "+criSocket+" mandatory to run cri tests:", err.Error())
	}

	engine, err := newCriEngine(context.Background(), criSocket)
	assert.NoError(t, err)

	id := uuid.New()
	podSandboxConfig := &v1.PodSandboxConfig{
		Metadata: &v1.PodSandboxMetadata{
			Name:      "test",
			Uid:       id.String(),
			Namespace: "default",
			Attempt:   0,
		},
	}
	sandboxName, err := client.RunPodSandbox(context.Background(), podSandboxConfig, "")
	assert.NoError(t, err)

	// Pull image
	imageClient, err := remote.NewRemoteImageService(criSocket, 20*time.Second, nil, nil)
	assert.NoError(t, err)
	imageSpec := &v1.ImageSpec{
		Image: "alpine:3.20.3",
	}
	if _, err = imageClient.ImageStatus(context.Background(), imageSpec, false); err != nil {
		_, err = imageClient.PullImage(context.Background(), imageSpec, nil, podSandboxConfig)
		assert.NoError(t, err)
	}

	ctr, err := client.CreateContainer(context.Background(), sandboxName, &v1.ContainerConfig{
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
				Privileged: false,
			},
		},
	}, podSandboxConfig)
	assert.NoError(t, err)

	events, err := engine.List(context.Background())
	assert.NoError(t, err)

	expectedEvent := Event{
		Info: Info{Container{
			Type:             typeContainerd.ToCTValue(),
			ID:               ctr[:shortIDLength],
			Name:             "test_container",
			Image:            "docker.io/library/alpine:3.20.3",
			ImageDigest:      "docker.io/library/alpine@sha256:1e42bbe2508154c9126d48c2b8a75420c3544343bf86fd041fb7527e017a4b4a",
			User:             "&ContainerUser{Linux:nil,}",
			CPUPeriod:        defaultCpuPeriod,
			CPUQuota:         2000,
			CPUShares:        defaultCpuShares,
			CPUSetCPUCount:   3,
			Env:              nil, // TODO
			FullID:           ctr,
			Labels:           map[string]string{"foo": "bar", "io.kubernetes.sandbox.id": sandboxName, "io.kubernetes.pod.name": "test", "io.kubernetes.pod.namespace": "default", "io.kubernetes.pod.uid": id.String()},
			PodSandboxID:     sandboxName,
			Privileged:       false, // TODO
			PodSandboxLabels: map[string]string{},
			Mounts:           []mount{},
			IsPodSandbox:     true,
		}},
		IsCreate: true,
	}

	found := false
	for _, event := range events {
		if event.FullID == ctr {
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

	err = client.RemoveContainer(context.Background(), "test_sandbox_test_container_0")
	assert.NoError(t, err)

	err = client.RemovePodSandbox(context.Background(), sandboxName)
	assert.NoError(t, err)

	// receive the "remove" event
	expectedEvent = Event{
		Info: Info{Container{
			Type:        typeContainerd.ToCTValue(),
			ID:          ctr[:shortIDLength],
			FullID:      ctr,
			CreatedTime: expectedEvent.CreatedTime,
		}},
		IsCreate: false,
	}
	for {
		event := waitOnChannelOrTimeout(t, listCh)
		if event.IsCreate == false {
			assert.Equal(t, expectedEvent, event)
			break
		}
	}
}
