package container

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/FedeDP/container-worker/pkg/config"
	"github.com/FedeDP/container-worker/pkg/event"
	"github.com/containers/podman/v5/libpod/define"
	"github.com/containers/podman/v5/pkg/bindings"
	"github.com/containers/podman/v5/pkg/bindings/containers"
	"github.com/containers/podman/v5/pkg/bindings/system"
	"github.com/containers/podman/v5/pkg/domain/entities/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/events"
	"strconv"
	"strings"
	"sync"
)

const typePodman engineType = "podman"

func init() {
	engineGenerators[typePodman] = newPodmanEngine
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

func (pc *podmanEngine) ctrToInfo(ctr *define.InspectContainerData) event.Info {
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

	mounts := make([]event.Mount, 0)
	for _, m := range ctr.Mounts {
		mounts = append(mounts, event.Mount{
			Source:      m.Source,
			Destination: m.Destination,
			Mode:        m.Mode,
			RW:          m.RW,
			Propagation: m.Propagation,
		})
	}

	portMappings := make([]event.PortMapping, 0)
	for port, portBindings := range netCfg.Ports {
		if !strings.Contains(port, "/tcp") {
			continue
		}
		containerPort, err := strconv.Atoi(port)
		if err != nil {
			continue
		}
		for _, portBinding := range portBindings {
			portMappings = append(portMappings, event.PortMapping{
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
	var (
		livenessProbe    *event.Probe = nil
		readinessProbe   *event.Probe = nil
		healthcheckProbe *event.Probe = nil
	)
	for key, val := range cfg.Labels {
		if len(val) <= config.GetLabelMaxLen() {
			labels[key] = val
		}
		if key == k8sLastAppliedConfigLabel {
			var k8sPodInfo k8sPodSpecInfo
			err := json.Unmarshal([]byte(val), &k8sPodInfo)
			if err == nil {
				if k8sPodInfo.Spec.Containers[0].LivenessProbe != nil {
					livenessProbe = parseLivenessReadinessProbe(k8sPodInfo.Spec.Containers[0].LivenessProbe)
				} else if k8sPodInfo.Spec.Containers[0].ReadinessProbe != nil {
					readinessProbe = parseLivenessReadinessProbe(k8sPodInfo.Spec.Containers[0].ReadinessProbe)
				}
			}
		}
	}
	if livenessProbe == nil && readinessProbe == nil && cfg.Healthcheck != nil {
		hConfig := container.HealthConfig{
			Test:          cfg.Healthcheck.Test,
			Interval:      cfg.Healthcheck.Interval,
			Timeout:       cfg.Healthcheck.Timeout,
			StartPeriod:   cfg.Healthcheck.StartPeriod,
			StartInterval: cfg.Healthcheck.StartInterval,
			Retries:       cfg.Healthcheck.Retries,
		}
		healthcheckProbe = parseHealthcheckProbe(&hConfig)
	}

	var (
		cpuShares int64 = defaultCpuShares
		cpuPeriod int64 = defaultCpuPeriod
	)
	if hostCfg.CpuShares > 0 {
		cpuShares = int64(hostCfg.CpuShares)
	}
	if hostCfg.CpuPeriod > 0 {
		cpuPeriod = int64(hostCfg.CpuPeriod)
	}
	cpusetCount := countCPUSet(hostCfg.CpusetCpus)

	var size int64 = -1
	if ctr.SizeRw != nil {
		size = *ctr.SizeRw
	}

	return event.Info{
		Container: event.Container{
			Type:             typePodman.ToCTValue(),
			ID:               shortContainerID(ctr.ID),
			Name:             name,
			Image:            ctr.ImageName,
			ImageDigest:      ctr.ImageDigest,
			ImageID:          ctr.Image,
			ImageRepo:        imageRepo,
			ImageTag:         imageTag,
			User:             cfg.User,
			CPUPeriod:        cpuPeriod,
			CPUQuota:         hostCfg.CpuQuota,
			CPUShares:        cpuShares,
			CPUSetCPUCount:   cpusetCount,
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
			Privileged:       hostCfg.Privileged,
			PortMappings:     portMappings,
			Mounts:           mounts,
			Size:             size,
			LivenessProbe:    livenessProbe,
			ReadinessProbe:   readinessProbe,
			HealthcheckProbe: healthcheckProbe,
		},
	}
}

func (pc *podmanEngine) Get(_ context.Context, containerId string) (*event.Event, error) {
	size := config.GetWithSize()
	ctrInfo, err := containers.Inspect(pc.pCtx, containerId, &containers.InspectOptions{Size: &size})
	if err != nil {
		return nil, err
	}
	return &event.Event{
		Info:     pc.ctrToInfo(ctrInfo),
		IsCreate: true,
	}, nil
}

func (pc *podmanEngine) List(_ context.Context) ([]event.Event, error) {
	evts := make([]event.Event, 0)
	all := true
	size := config.GetWithSize()
	cList, err := containers.List(pc.pCtx, &containers.ListOptions{All: &all})
	if err != nil {
		return nil, err
	}
	for _, c := range cList {
		ctrInfo, err := containers.Inspect(pc.pCtx, c.ID, &containers.InspectOptions{Size: &size})
		if err != nil {
			evts = append(evts, event.Event{
				Info: event.Info{
					Container: event.Container{
						Type:        typePodman.ToCTValue(),
						ID:          shortContainerID(c.ID),
						Image:       c.Image,
						FullID:      c.ID,
						ImageID:     c.ImageID,
						CreatedTime: c.Created.Unix(),
					},
				},
				IsCreate: true,
			})
		} else {
			evts = append(evts, event.Event{
				Info:     pc.ctrToInfo(ctrInfo),
				IsCreate: true,
			})
		}

	}
	return evts, nil
}

func (pc *podmanEngine) Listen(ctx context.Context, wg *sync.WaitGroup) (<-chan event.Event, error) {
	stream := true
	filters := map[string][]string{
		"type": {string(events.ContainerEventType)},
		"event": {
			string(events.ActionCreate),
			string(events.ActionRemove),
		},
	}
	evChn := make(chan types.Event)
	cancelChan := make(chan bool)
	wg.Add(1)
	// producers
	go func(ch chan types.Event) {
		defer wg.Done()
		_ = system.Events(pc.pCtx, ch, cancelChan, &system.EventsOptions{
			Filters: filters,
			Stream:  &stream,
		})
	}(evChn)

	outCh := make(chan event.Event)
	wg.Add(1)
	go func() {
		defer close(outCh)
		defer close(cancelChan)
		defer wg.Done()
		size := config.GetWithSize()
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
					ctr, err = containers.Inspect(pc.pCtx, ev.Actor.ID, &containers.InspectOptions{Size: &size})
				}
				if err != nil {
					// At least send an event with the minimal set of data
					outCh <- event.Event{
						Info: event.Info{
							Container: event.Container{
								Type:   typePodman.ToCTValue(),
								ID:     shortContainerID(ev.Actor.ID),
								FullID: ev.Actor.ID,
								Image:  ev.Actor.Attributes["image"],
							},
						},
						IsCreate: ev.Action == events.ActionCreate,
					}
				} else {
					outCh <- event.Event{
						Info:     pc.ctrToInfo(ctr),
						IsCreate: ev.Action == events.ActionCreate,
					}
				}
			}
		}
	}()
	return outCh, nil
}
