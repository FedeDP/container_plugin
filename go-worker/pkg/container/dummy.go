package container

import (
	"context"
	"github.com/FedeDP/container-worker/pkg/event"
	"sync"
)

var dummyChan chan string

func GetDummyChan() chan<- string {
	return dummyChan
}

type dummy struct {
	engines []Engine
}

// NewDummyEngine returns a dummy engine.
// The dummy engine is responsible to allow us to Get() single container
// trying all container engines enabled.
func NewDummyEngine(containerEngines []Engine) Engine {
	return &dummy{engines: containerEngines}
}

func (d *dummy) Get(_ context.Context, _ string) (*event.Event, error) {
	return nil, nil
}

func (d *dummy) List(_ context.Context) ([]event.Event, error) {
	return nil, nil
}

func (d *dummy) Listen(ctx context.Context, wg *sync.WaitGroup) (<-chan event.Event, error) {
	outCh := make(chan event.Event)
	wg.Add(1)
	dummyChan = make(chan string)
	go func() {
		defer close(outCh)
		defer wg.Done()
		for {
			select {
			case <-ctx.Done():
				return
			case containerId := <-dummyChan:
				for _, e := range d.engines {
					evt, _ := e.Get(ctx, containerId)
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
