package clients

import (
	"context"
	"errors"
	"github.com/containers/podman/v5/pkg/bindings"
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
	return &PodmanClient{podmanContexts}, nil
}

func (pc *PodmanClient) List(_ context.Context) ([]Info, error) {
	//TODO implement me
	return nil, nil
}

func (pc *PodmanClient) Listener(_ context.Context) (Listener, error) {
	//TODO implement me
	ch := make(chan string)
	return ch, nil
}
