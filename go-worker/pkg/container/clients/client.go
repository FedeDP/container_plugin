package clients

import (
	"context"
)

type Info struct {
	ID    string `json:"id"`
	Image string `json:"image"`
	State string `json:"state"`
}

type Client interface {
	List(ctx context.Context) ([]Info, error)
	Listener(ctx context.Context) (Listener, error)
}

type Listener <-chan string
