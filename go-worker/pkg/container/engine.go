package container

import (
	"context"
	"encoding/json"
)

var Engines = make(map[Type]Engine)

type Type string

// TODO: add other needed fields
type Info struct {
	Type  string `json:"type"`
	ID    string `json:"id"`
	Image string `json:"image"`
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

type Engine interface {
	// Init initializes private engine data
	Init(ctx context.Context) error
	// List lists all running container for the engine
	List(ctx context.Context) ([]Event, error)
	// Listen returns a channel where container created/deleted events will be notified
	Listen(ctx context.Context) (<-chan Event, error)
}
