package container

import (
	"context"
	"github.com/FedeDP/container-worker/pkg/event"
	"sync"
)

/*
Fetcher is a fake engine that listens on fetcherChan for published containerIDs.
Everytime a containerID is published on the channel, the fetcher engine loops
over all enabled engines and tries to get info about the container,
until it succeeds and publish an event to the output channel.
FetcherChan requests are published through a CGO exposed API: AskForContainerInfo(), in worker_api.
*/

var fetcherChan chan string

func GetFetcherChan() chan<- string {
	return fetcherChan
}

type fetcher struct {
	getters []getter
}

// NewFetcherEngine returns a fetcher engine.
// The fetcher engine is responsible to allow us to get() single container
// trying all container engines enabled.
func NewFetcherEngine(containerEngines []Engine) Engine {
	f := fetcher{
		getters: make([]getter, len(containerEngines)),
	}
	for i, e := range containerEngines {
		// No type check since Engine interface extends getter.
		f.getters[i] = e.(getter)
	}
	return &f
}

func (f *fetcher) get(_ context.Context, _ string) (*event.Event, error) {
	return nil, nil
}

func (f *fetcher) List(_ context.Context) ([]event.Event, error) {
	return nil, nil
}

func (f *fetcher) Listen(ctx context.Context, wg *sync.WaitGroup) (<-chan event.Event, error) {
	outCh := make(chan event.Event)
	wg.Add(1)
	fetcherChan = make(chan string)
	go func() {
		defer close(outCh)
		defer wg.Done()
		for {
			select {
			case <-ctx.Done():
				return
			case containerId := <-fetcherChan:
				for _, e := range f.getters {
					evt, _ := e.get(ctx, containerId)
					if evt != nil {
						outCh <- *evt
						break
					}
				}
			}
		}
	}()
	return outCh, nil
}
