package clients

import (
	"context"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/api/types/filters"
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

func (dc *DockerClient) List(ctx context.Context) ([]Event, error) {
	containers, err := dc.ContainerList(ctx, container.ListOptions{})
	if err != nil {
		return nil, err
	}

	evts := make([]Event, len(containers))
	idx := 0
	for _, ctr := range containers {
		evts[idx] = Event{
			IsCreate: true,
			Info: Info{
				Type:  "docker",
				ID:    ctr.ID,
				Image: ctr.Image,
				State: ctr.State,
			},
		}
		idx++
	}
	return evts, nil
}

func (dc *DockerClient) Listen(ctx context.Context) (<-chan Event, error) {
	outCh := make(chan Event)

	flts := filters.NewArgs()
	flts.Add("type", string(events.ContainerEventType))
	flts.Add("event", string(events.ActionCreate))
	flts.Add("event", string(events.ActionDestroy))
	msgs, _ := dc.Events(ctx, events.ListOptions{Filters: flts})
	go func() {
		defer close(outCh)
		for msg := range msgs {
			outCh <- Event{
				Info: Info{
					Type:  "docker",
					ID:    msg.Actor.ID,
					Image: msg.Actor.Attributes["image"],
					State: string(msg.Action),
				},
				IsCreate: msg.Action == events.ActionCreate,
			}
		}
	}()
	return outCh, nil
}