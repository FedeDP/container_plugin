package container

import (
	"context"
	"errors"
	"github.com/containers/podman/v5/libpod/define"
	"github.com/containers/podman/v5/pkg/bindings"
	"github.com/containers/podman/v5/pkg/bindings/containers"
	"github.com/containers/podman/v5/pkg/bindings/system"
	"github.com/containers/podman/v5/pkg/domain/entities/types"
	"github.com/docker/docker/api/types/events"
	"strconv"
	"strings"
)

const typePodman Type = "podman"

func init() {
	EngineGenerators[typePodman] = newPodmanEngine
}

type podmanEngine struct {
	pCtx context.Context
}

func newPodmanEngine(ctx context.Context, socket string) (Engine, error) {
	conn, err := bindings.NewConnection(ctx, enforceUnixProtocolIfEmpty(socket))
	if err != nil {
		return nil, err
	}
	return &podmanEngine{conn}, nil
}

func (pc *podmanEngine) ctrToInfo(ctr *define.InspectContainerData) Info {
	cfg := ctr.Config
	if cfg == nil {
		cfg = &define.InspectContainerConfig{}
	}
	hostCfg := ctr.HostConfig
	if hostCfg == nil {
		hostCfg = &define.InspectContainerHostConfig{}
	}
	netCfg := ctr.NetworkSettings
	if netCfg == nil {
		netCfg = &define.InspectNetworkSettings{}
	}
	var name string
	isPodSandbox := false
	name = strings.TrimPrefix(ctr.Name, "/")
	isPodSandbox = strings.Contains(name, "k8s_POD")

	mounts := make([]Mount, 0)
	for _, m := range ctr.Mounts {
		mounts = append(mounts, Mount{
			Source:      m.Source,
			Destination: m.Destination,
			Mode:        m.Mode,
			RW:          m.RW,
			Propagation: m.Propagation,
		})
	}

	portMappings := make([]PortMapping, 0)
	for port, portBindings := range netCfg.Ports {
		if !strings.Contains(port, "/tcp") {
			continue
		}
		containerPort, err := strconv.Atoi(port)
		if err != nil {
			continue
		}
		for _, portBinding := range portBindings {
			portMappings = append(portMappings, PortMapping{
				HostIp:        portBinding.HostIP,
				HostPort:      portBinding.HostPort,
				ContainerPort: containerPort,
			})
		}
	}

	var (
		imageRepo string
		imageTag  string
	)
	imageRepoTag := strings.Split(ctr.ImageName, ":")
	if len(imageRepoTag) == 2 {
		imageRepo = imageRepoTag[0]
		imageTag = imageRepoTag[1]
	}

	labels := make(map[string]string)
	for key, val := range cfg.Labels {
		if len(val) <= maxLabelLength {
			labels[key] = val
		}
	}

	return Info{
		Type:             string(typePodman),
		ID:               ctr.ID[:12],
		Name:             name,
		Image:            ctr.ImageName,
		ImageDigest:      ctr.ImageDigest,
		ImageID:          ctr.Image,
		ImageRepo:        imageRepo,
		ImageTag:         imageTag,
		User:             cfg.User,
		CniJson:          "", // TODO
		CPUPeriod:        int64(hostCfg.CpuPeriod),
		CPUQuota:         hostCfg.CpuQuota,
		CPUShares:        int64(hostCfg.CpuShares),
		CPUSetCPUCount:   int64(hostCfg.CpuCount),
		CreatedTime:      ctr.Created.Unix(),
		Env:              cfg.Env,
		FullID:           ctr.ID,
		HostIPC:          hostCfg.IpcMode == "host",
		HostNetwork:      hostCfg.NetworkMode == "host",
		HostPID:          hostCfg.PidMode == "host",
		Ip:               netCfg.IPAddress,
		IsPodSandbox:     isPodSandbox,
		Labels:           labels,
		MemoryLimit:      hostCfg.Memory,
		SwapLimit:        hostCfg.MemorySwap,
		MetadataDeadline: 0,                // TODO
		PodSandboxID:     netCfg.SandboxID, // TODO double check
		Privileged:       hostCfg.Privileged,
		PodSandboxLabels: nil, // TODO
		PortMappings:     portMappings,
		Mounts:           mounts,
	}
}

func (pc *podmanEngine) List(_ context.Context) ([]Event, error) {
	evts := make([]Event, 0)
	all := true
	cList, err := containers.List(pc.pCtx, &containers.ListOptions{All: &all})
	if err != nil {
		return nil, err
	}
	for _, c := range cList {
		ctrInfo, err := containers.Inspect(pc.pCtx, c.ID, nil)
		if err != nil {
			evts = append(evts, Event{
				Info: Info{
					Type:        string(typePodman),
					ID:          c.ID[:12],
					Image:       c.Image,
					FullID:      c.ID,
					ImageID:     c.ImageID,
					CreatedTime: c.Created.Unix(),
				},
				IsCreate: true,
			})
		} else {
			evts = append(evts, Event{
				Info:     pc.ctrToInfo(ctrInfo),
				IsCreate: true,
			})
		}

	}
	return evts, nil
}

func (pc *podmanEngine) Listen(ctx context.Context) (<-chan Event, error) {
	stream := true
	filters := map[string][]string{
		"type": {string(events.ContainerEventType)},
		"event": {
			string(events.ActionCreate),
			string(events.ActionRemove),
		},
	}
	evChn := make(chan types.Event)
	// producers
	go func(ch chan types.Event) {
		_ = system.Events(pc.pCtx, ch, nil, &system.EventsOptions{
			Filters: filters,
			Stream:  &stream,
		})
	}(evChn)

	outCh := make(chan Event)
	go func() {
		defer close(outCh)
		// Blocking: convert all events from podman to json strings
		// and send them to the main loop until the channel is closed
		for {
			select {
			case <-ctx.Done():
				return
			case ev := <-evChn:
				err := errors.New("inspect useless on action destroy")
				ctr := &define.InspectContainerData{}
				if ev.Action == events.ActionCreate {
					ctr, err = containers.Inspect(pc.pCtx, ev.Actor.ID, nil)
				}
				if err != nil {
					// At least send an event with the minimal set of data
					outCh <- Event{
						Info: Info{
							Type:   string(typePodman),
							ID:     ev.Actor.ID[:12],
							FullID: ev.Actor.ID,
							Image:  ev.Actor.Attributes["image"],
						},
						IsCreate: ev.Action == events.ActionCreate,
					}
				} else {
					outCh <- Event{
						Info:     pc.ctrToInfo(ctr),
						IsCreate: ev.Action == events.ActionCreate,
					}
				}
			}
		}
	}()
	return outCh, nil
}
