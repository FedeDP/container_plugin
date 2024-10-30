package clients

import (
	"context"
	"errors"
	"fmt"
	"github.com/containers/podman/v5/libpod/events"
	"github.com/containers/podman/v5/pkg/bindings"
	"github.com/containers/podman/v5/pkg/bindings/containers"
	"github.com/containers/podman/v5/pkg/domain/entities"
	"github.com/containers/podman/v5/pkg/domain/infra"
	"os"
)

type PodmanClient struct {
	ctxs []context.Context
}

func NewPodmanClient(ctx context.Context) (Client, error) {
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
		return nil, errors.New("no podman context found")
	}
	return &PodmanClient{
		ctxs: podmanContexts,
	}, nil
}

func (pc *PodmanClient) List(_ context.Context) ([]Event, error) {
	evts := make([]Event, 0)
	for _, ctx := range pc.ctxs {
		cList, err := containers.List(ctx, &containers.ListOptions{})
		if err != nil {
			continue
		}
		for _, c := range cList {
			evts = append(evts, Event{
				Info: Info{
					Type:  "podman",
					ID:    c.ID,
					Image: c.Image,
					State: c.State,
				},
				IsCreate: true,
			})
		}
	}
	return evts, nil
}

func (pc *PodmanClient) Listen(ctx context.Context) (<-chan Event, error) {
	// FIXME
	return nil, nil
	evCh := make(chan *events.Event)
	outCh := make(chan Event)
	engine, err := infra.NewContainerEngine(&entities.PodmanConfig{
		EngineMode: entities.ABIMode,
	})
	if err != nil {
		fmt.Println(err.Error())
		return nil, err
	}
	go func() {
		defer close(evCh)
		// Blocking, read all events from podman
		_ = engine.Events(ctx, entities.EventsOptions{
			EventChan: evCh,
			Filter:    nil,
			Stream:    true,
		})
	}()
	go func() {
		defer close(outCh)
		// Blocking: convert all events from podman to json strings
		// and send them to the main loop until the channel is closed
		for t := range evCh {
			outCh <- Event{
				Info: Info{
					Type:  "podman",
					ID:    t.ID,
					Image: t.Image,
					State: t.Status.String(),
				},
				IsCreate: t.Status == events.Create,
			}
		}
	}()
	return outCh, nil
}
