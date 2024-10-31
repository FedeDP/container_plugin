package container

import (
	"context"
	"errors"
	"github.com/containers/podman/v5/pkg/bindings"
	"github.com/containers/podman/v5/pkg/bindings/containers"
	"github.com/containers/podman/v5/pkg/bindings/system"
	"github.com/containers/podman/v5/pkg/domain/entities/types"
	"github.com/docker/docker/api/types/events"
	"os"
	"reflect"
)

const typePodman Type = "podman"

func init() {
	Engines[typePodman] = &podmanEngine{}
}

type podmanEngine struct {
	ctxs []context.Context
}

func (pc *podmanEngine) Init(ctx context.Context) error {
	podmanContexts := make([]context.Context, 0)
	// Get root podman socket location
	rootSocket := "unix:///run/podman/podman.sock"
	rootConn, rootErr := bindings.NewConnection(ctx, rootSocket)
	if rootErr == nil {
		podmanContexts = append(podmanContexts, rootConn)
	}

	// Get all user podman sockets
	items, _ := os.ReadDir("/run/user/")
	for _, item := range items {
		userSocket := "unix://" + "/run/user/" + item.Name() + "/podman/podman.sock"
		// Connect to Podman socket
		userConn, userErr := bindings.NewConnection(ctx, userSocket)
		if userErr == nil {
			podmanContexts = append(podmanContexts, userConn)
		}
	}

	if len(podmanContexts) == 0 {
		return errors.New("no podman context found")
	}
	pc.ctxs = podmanContexts
	return nil
}

func (pc *podmanEngine) List(_ context.Context) ([]Event, error) {
	evts := make([]Event, 0)
	for _, ctx := range pc.ctxs {
		all := true
		cList, err := containers.List(ctx, &containers.ListOptions{All: &all})
		if err != nil {
			continue
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
	}
	return evts, nil
}

func (pc *podmanEngine) Listen(ctx context.Context) (<-chan Event, error) {
	outCh := make(chan Event)

	// We need to use a reflect.SelectCase here since
	// we will need to select a variable number of channels,
	// depending on how many podman contexts are found in the system.
	cases := make([]reflect.SelectCase, len(pc.ctxs)+1) // for the ctx.Done case

	stream := true
	filters := map[string][]string{
		"type": {string(events.ContainerEventType)},
		"event": {
			string(events.ActionCreate),
			string(events.ActionRemove),
		},
	}
	for i, c := range pc.ctxs {
		evChn := make(chan types.Event)
		// producers
		go func(ch chan types.Event) {
			_ = system.Events(c, ch, nil, &system.EventsOptions{
				Filters: filters,
				Stream:  &stream,
			})
		}(evChn)

		cases[i] = reflect.SelectCase{
			Dir:  reflect.SelectRecv,
			Chan: reflect.ValueOf(evChn),
		}
	}

	// Emplace back case for `ctx.Done` channel
	cases[len(pc.ctxs)] = reflect.SelectCase{
		Dir:  reflect.SelectRecv,
		Chan: reflect.ValueOf(ctx.Done()),
	}

	// consumer/adapter/producer
	go func() {
		defer close(outCh)
		// Blocking: convert all events from podman to json strings
		// and send them to the main loop until the channel is closed
		for {
			chosen, val, _ := reflect.Select(cases)
			if chosen == len(pc.ctxs) {
				// ctx.Done!
				return
			} else {
				ev, _ := val.Interface().(types.Event)
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
