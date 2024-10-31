package container

import (
	"context"
	internalapi "k8s.io/cri-api/pkg/apis"
	v1 "k8s.io/cri-api/pkg/apis/runtime/v1"
	remote "k8s.io/cri-client/pkg"
	"time"
)

const typeCri Type = "cri"

func init() {
	Engines[typeCri] = &criEngine{}
}

type criEngine struct {
	client  internalapi.RuntimeService
	runtime string
}

// See https://github.com/falcosecurity/libs/blob/4d04cad02cd27e53cb18f431361a4d031836bb75/userspace/libsinsp/cri.hpp#L71
func getRuntime(runtime string) string {
	if runtime == "containerd" || runtime == "cri-o" {
		return runtime
	}
	return string(typeCri)
}

func (c *criEngine) Init(ctx context.Context) error {
	client, err := remote.NewRemoteRuntimeService("/run/containerd/containerd.sock", 5*time.Second, nil, nil)
	if err != nil {
		return err
	}
	version, err := client.Version(ctx, "")
	if err != nil {
		return err
	}
	c.client = client
	c.runtime = getRuntime(version.RuntimeName)
	return nil
}

func (c *criEngine) List(ctx context.Context) ([]Event, error) {
	ctrs, err := c.client.ListContainers(ctx, &v1.ContainerFilter{State: &v1.ContainerStateValue{}})
	if err != nil {
		return nil, err
	}
	evts := make([]Event, len(ctrs))
	for idx, ctr := range ctrs {
		image := ctr.GetImage()
		var img string
		if image != nil {
			img = image.Image
		}
		evts[idx] = Event{
			IsCreate: true,
			Info: Info{
				Type:  c.runtime,
				ID:    ctr.Id,
				Image: img,
			},
		}
	}
	return evts, nil
}

func (c *criEngine) Listen(ctx context.Context) (<-chan Event, error) {
	containerEventsCh := make(chan *v1.ContainerEventResponse)
	go func() {
		_ = c.client.GetContainerEvents(ctx, containerEventsCh, nil)
	}()
	outCh := make(chan Event)
	go func() {
		defer close(outCh)
		for event := range containerEventsCh {
			if event.ContainerEventType == v1.ContainerEventType_CONTAINER_CREATED_EVENT ||
				event.ContainerEventType == v1.ContainerEventType_CONTAINER_DELETED_EVENT {
				cStatus := event.GetContainersStatuses()
				var img string
				if len(cStatus) > 0 {
					image := cStatus[0].GetImage()
					if image != nil {
						img = image.Image
					}
				}
				outCh <- Event{
					Info: Info{
						Type:  c.runtime,
						ID:    event.ContainerId,
						Image: img,
					},
					IsCreate: event.ContainerEventType == v1.ContainerEventType_CONTAINER_CREATED_EVENT,
				}
			}
		}
	}()
	return outCh, nil
}
