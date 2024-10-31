package container

import (
	"context"
	"github.com/containers/podman/v5/pkg/bindings"
	"github.com/containers/podman/v5/pkg/bindings/containers"
	"github.com/containers/podman/v5/pkg/bindings/system"
	"github.com/containers/podman/v5/pkg/domain/entities/types"
	"github.com/docker/docker/api/types/events"
)

const typePodman Type = "podman"

func init() {
	EngineGenerators[typePodman] = newPodmanEngine
}

type podmanEngine struct {
	pCtx context.Context
}

func newPodmanEngine(ctx context.Context, socket string) (Engine, error) {
	conn, err := bindings.NewConnection(ctx, enforceUnixProtocolIfEmpty(socket))
	if err != nil {
		return nil, err
	}
	return &podmanEngine{conn}, nil
}

func (pc *podmanEngine) List(_ context.Context) ([]Event, error) {
	evts := make([]Event, 0)
	all := true
	cList, err := containers.List(pc.pCtx, &containers.ListOptions{All: &all})
	if err != nil {
		return nil, err
	}
	for _, c := range cList {
		evts = append(evts, Event{
			Info: Info{
				Type:  string(typePodman),
				ID:    c.ID,
				Image: c.Image,
			},
			IsCreate: true,
		})
	}
	return evts, nil
}

func (pc *podmanEngine) Listen(ctx context.Context) (<-chan Event, error) {
	stream := true
	filters := map[string][]string{
		"type": {string(events.ContainerEventType)},
		"event": {
			string(events.ActionCreate),
			string(events.ActionRemove),
		},
	}
	evChn := make(chan types.Event)
	// producers
	go func(ch chan types.Event) {
		_ = system.Events(pc.pCtx, ch, nil, &system.EventsOptions{
			Filters: filters,
			Stream:  &stream,
		})
	}(evChn)

	outCh := make(chan Event)
	go func() {
		defer close(outCh)
		// Blocking: convert all events from podman to json strings
		// and send them to the main loop until the channel is closed
		for {
			select {
			case <-ctx.Done():
				return
			case ev := <-evChn:
				outCh <- Event{
					Info: Info{
						Type:  string(typePodman),
						ID:    ev.Actor.ID,
						Image: ev.Actor.Attributes["image"],
					},
					IsCreate: ev.Action == events.ActionCreate,
				}
			}
		}
	}()
	return outCh, nil
}
