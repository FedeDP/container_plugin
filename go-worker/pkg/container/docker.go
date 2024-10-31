package container

import (
	"context"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
)

const typeDocker Type = "docker"

func init() {
	Engines[typeDocker] = &dockerEngine{}
}

type dockerEngine struct {
	*client.Client
}

func (dc *dockerEngine) Init(_ context.Context) error {
	cl, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return err
	}
	dc.Client = cl
	return nil
}

func (dc *dockerEngine) List(ctx context.Context) ([]Event, error) {
	containers, err := dc.ContainerList(ctx, container.ListOptions{All: true})
	if err != nil {
		return nil, err
	}

	evts := make([]Event, len(containers))
	for idx, ctr := range containers {
		evts[idx] = Event{
			IsCreate: true,
			Info: Info{
				Type:  string(typeDocker),
				ID:    ctr.ID,
				Image: ctr.Image,
			},
		}
	}
	return evts, nil
}

func (dc *dockerEngine) Listen(ctx context.Context) (<-chan Event, error) {
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
					Type:  string(typeDocker),
					ID:    msg.Actor.ID,
					Image: msg.Actor.Attributes["image"],
				},
				IsCreate: msg.Action == events.ActionCreate,
			}
		}
	}()
	return outCh, nil
}
