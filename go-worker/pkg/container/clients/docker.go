package clients

import (
	"context"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
)

type DockerClient struct {
	*client.Client
}

func NewDockerClient(_ context.Context) (Client, error) {
	cl, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, err
	}

	dCl := &DockerClient{cl}
	return dCl, nil
}

func (dc *DockerClient) List(ctx context.Context) ([]Info, error) {
	containers, err := dc.ContainerList(ctx, container.ListOptions{})
	if err != nil {
		return nil, err
	}

	infos := make([]Info, 0, len(containers))
	idx := 0
	for _, ctr := range containers {
		infos[idx] = Info{
			ID:    ctr.ID,
			Image: ctr.Image,
			State: ctr.State,
		}
		idx++
	}
	return infos, nil
}

func (dc *DockerClient) Listener(ctx context.Context) (Listener, error) {
	//msgs, errs := dc.Events(ctx, events.ListOptions{})
	ch := make(chan string)
	return ch, nil
}
