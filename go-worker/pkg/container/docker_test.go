package container

import (
	"context"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
	"github.com/stretchr/testify/assert"
	"io"
	"testing"
)

func TestDocker(t *testing.T) {
	dockerClient, err := client.NewClientWithOpts(client.FromEnv,
		client.WithAPIVersionNegotiation())
	if err != nil {
		t.Skip("Socket " + client.DefaultDockerHost + " mandatory to run docker tests")
	}

	engine, err := newDockerEngine(context.Background(), client.DefaultDockerHost)
	assert.NoError(t, err)

	if _, _, err = dockerClient.ImageInspectWithRaw(context.Background(), "alpine:3.20.3"); client.IsErrNotFound(err) {
		pullRes, err := dockerClient.ImagePull(context.Background(), "alpine:3.20.3", image.PullOptions{})
		assert.NoError(t, err)

		defer pullRes.Close()
		_, err = io.Copy(io.Discard, pullRes)
		assert.NoError(t, err)
	}

	ctr, err := dockerClient.ContainerCreate(context.Background(), &container.Config{
		User:   "testuser",
		Env:    []string{"env=env"},
		Image:  "alpine:3.20.3",
		Labels: map[string]string{"foo": "bar"},
	}, &container.HostConfig{
		Privileged: true,
		Resources: container.Resources{
			CPUQuota:   2000,
			CpusetCpus: "0-1",
		},
	}, nil, nil, "test_container")
	assert.NoError(t, err)

	events, err := engine.List(context.Background())
	assert.NoError(t, err)

	expectedEvent := Event{
		Info: Info{Container{
			Type:           typeDocker.ToCTValue(),
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
			CPUSetCPUCount: 2, // 0-1
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

	err = dockerClient.ContainerRemove(context.Background(), ctr.ID, container.RemoveOptions{})
	assert.NoError(t, err)

	// receive the "remove" event
	expectedEvent = Event{
		Info: Info{Container{
			Type:   typeDocker.ToCTValue(),
			ID:     ctr.ID[:shortIDLength],
			FullID: ctr.ID,
			Image:  "alpine:3.20.3",
		}},
		IsCreate: false,
	}

	event := <-listCh
	assert.Equal(t, expectedEvent, event)
}
