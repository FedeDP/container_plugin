package container

import (
	"context"
	"github.com/containerd/containerd/api/events"
	containerd "github.com/containerd/containerd/v2/client"
	"github.com/containerd/containerd/v2/pkg/namespaces"
	"github.com/containerd/typeurl/v2"
)

const typeContainerd Type = "containerd"

func init() {
	EngineGenerators[typeContainerd] = newContainerdEngine
}

type containerdEngine struct {
	client *containerd.Client
}

func newContainerdEngine(_ context.Context, socket string) (Engine, error) {
	client, err := containerd.New(socket)
	if err != nil {
		return nil, err
	}
	return &containerdEngine{client: client}, nil
}

func (c *containerdEngine) List(ctx context.Context) ([]Event, error) {
	namespacesList, err := c.client.NamespaceService().List(ctx)
	if err != nil {
		return nil, err
	}
	evts := make([]Event, 0)
	for _, namespace := range namespacesList {
		namespacedContext := namespaces.WithNamespace(ctx, namespace)
		containers, err := c.client.Containers(namespacedContext)
		if err != nil {
			continue
		}
		for _, container := range containers {
			image, err := container.Image(namespacedContext)
			var img string
			if err == nil {
				// No image on the deleted event
				img = image.Name()
			}
			evts = append(evts, Event{
				Info: Info{
					Type:  string(typeContainerd),
					ID:    container.ID(),
					Image: img,
				},
				IsCreate: true,
			})
		}
	}
	return evts, nil
}

func (c *containerdEngine) Listen(ctx context.Context) (<-chan Event, error) {
	outCh := make(chan Event)
	eventsClient := c.client.EventService()
	eventsCh, _ := eventsClient.Subscribe(ctx,
		`topic=="/containers/create"`, `topic=="/containers/delete"`)
	go func() {
		defer close(outCh)
		for ev := range eventsCh {
			ctrCreate := events.ContainerCreate{
				ID:      "",
				Image:   "",
				Runtime: nil,
			}
			err := typeurl.UnmarshalTo(ev.Event, &ctrCreate)
			if err == nil {
				outCh <- Event{
					Info: Info{
						Type:  string(typeContainerd),
						ID:    ctrCreate.ID,
						Image: ctrCreate.Image,
					},
					IsCreate: true,
				}
			} else {
				ctrDelete := events.ContainerDelete{}
				err = typeurl.UnmarshalTo(ev.Event, &ctrDelete)
				if err == nil {
					outCh <- Event{
						Info: Info{
							Type:  string(typeContainerd),
							ID:    ctrDelete.ID,
							Image: "",
						},
						IsCreate: false,
					}
				}
			}
		}
	}()
	return outCh, nil
}
