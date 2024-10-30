package clients

import (
	"context"
	"encoding/json"
)

type Info struct {
	Type  string `json:"type"`
	ID    string `json:"id"`
	Image string `json:"image"`
	State string `json:"state"`
}

type Event struct {
	Info
	IsCreate bool
}

func (i *Info) String() string {
	str, err := json.Marshal(i)
	if err != nil {
		return ""
	}
	return string(str)
}

type Client interface {
	List(ctx context.Context) ([]Event, error)
	Listen(ctx context.Context) (<-chan Event, error)
}
