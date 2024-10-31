package container

import (
	"context"
	"encoding/json"
	"net/url"
)

type Type string
type EngineGenerator func(context.Context, string) (Engine, error)

var EngineGenerators = make(map[Type]EngineGenerator)

type SocketsEngine struct {
	Enabled bool     `json:"enabled"`
	Sockets []string `json:"sockets"`
}

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
	// List lists all running container for the engine
	List(ctx context.Context) ([]Event, error)
	// Listen returns a channel where container created/deleted events will be notified
	Listen(ctx context.Context) (<-chan Event, error)
}

func enforceUnixProtocolIfEmpty(socket string) string {
	base, _ := url.Parse(socket)
	if base.Scheme == "" {
		base.Scheme = "unix"
		return base.String()
	}
	return socket
}
