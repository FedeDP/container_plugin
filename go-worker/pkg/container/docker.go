package container

import (
	"context"
	"errors"
	"github.com/FedeDP/container-worker/pkg/config"
	"github.com/FedeDP/container-worker/pkg/event"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
	"strings"
	"sync"
	"time"
)

const typeDocker engineType = "docker"

func init() {
	engineGenerators[typeDocker] = newDockerEngine
}

type dockerEngine struct {
	*client.Client
}

func newDockerEngine(_ context.Context, socket string) (Engine, error) {
	cl, err := client.NewClientWithOpts(client.FromEnv,
		client.WithAPIVersionNegotiation(),
		client.WithHost(enforceUnixProtocolIfEmpty(socket)))
	if err != nil {
		return nil, err
	}
	return &dockerEngine{cl}, nil
}

func (dc *dockerEngine) ctrToInfo(ctx context.Context, ctr types.ContainerJSON) event.Info {
	hostCfg := ctr.HostConfig
	if hostCfg == nil {
		hostCfg = &container.HostConfig{
			Resources: container.Resources{
				CPUPeriod: defaultCpuPeriod,
				CPUShares: defaultCpuShares,
			},
		}
	}
	mounts := make([]event.Mount, 0)
	for _, m := range ctr.Mounts {
		mounts = append(mounts, event.Mount{
			Source:      m.Source,
			Destination: m.Destination,
			Mode:        m.Mode,
			RW:          m.RW,
			Propagation: string(m.Propagation),
		})
	}

	var name string
	isPodSandbox := false
	name = strings.TrimPrefix(ctr.Name, "/")
	isPodSandbox = strings.Contains(name, "k8s_POD")

	netCfg := ctr.NetworkSettings
	if netCfg == nil {
		netCfg = &types.NetworkSettings{}
	}
	portMappings := make([]event.PortMapping, 0)
	for port, portBindings := range netCfg.Ports {
		if port.Proto() != "tcp" {
			continue
		}
		containerPort := port.Int()
		for _, portBinding := range portBindings {
			portMappings = append(portMappings, event.PortMapping{
				HostIp:        portBinding.HostIP,
				HostPort:      portBinding.HostPort,
				ContainerPort: containerPort,
			})
		}
	}
	cfg := ctr.Config
	if cfg == nil {
		cfg = &container.Config{}
	}

	image, _, err := dc.ImageInspectWithRaw(ctx, ctr.Image)
	if err != nil {
		image = types.ImageInspect{}
	}

	var (
		imageDigest string
		imageRepo   string
		imageTag    string
		imageID     string
	)
	imageDigestSet := make([]string, 0)
	for _, repoDigest := range image.RepoDigests {
		repoDigestParts := strings.Split(repoDigest, "@")
		if len(repoDigestParts) != 2 {
			// malformed
			continue
		}
		if imageRepo == "" {
			imageRepo = repoDigestParts[0]
		}
		digest := repoDigestParts[1]
		imageDigestSet = append(imageDigestSet, digest)
		if strings.Contains(repoDigest, imageRepo) {
			imageDigest = digest
			break
		}
	}
	if len(imageDigest) == 0 && len(imageDigestSet) == 1 {
		imageDigest = imageDigestSet[0]
	}

	for _, repoTag := range image.RepoTags {
		repoTagsParts := strings.Split(repoTag, ":")
		if len(repoTagsParts) != 2 {
			// malformed
			continue
		}
		if imageRepo == "" {
			imageRepo = repoTagsParts[0]
		}
		if strings.Contains(repoTag, imageRepo) {
			imageTag = repoTagsParts[1]
			break
		}
	}

	img := ctr.Image
	if !strings.Contains(img, "/") && strings.Contains(img, ":") {
		imageID = strings.Split(img, ":")[1]
	}

	labels := make(map[string]string)
	for key, val := range cfg.Labels {
		if len(val) <= config.GetLabelMaxLen() {
			labels[key] = val
		}
	}

	ip := netCfg.IPAddress
	if ip == "" {
		if hostCfg.NetworkMode.IsContainer() {
			secondaryID := hostCfg.NetworkMode.ConnectedContainer()
			secondary, _ := dc.ContainerInspect(ctx, secondaryID)
			if secondary.NetworkSettings != nil {
				ip = secondary.NetworkSettings.IPAddress
			}
		}
	}

	createdTime, _ := time.Parse(time.RFC3339Nano, ctr.Created)

	var (
		cpuShares int64 = defaultCpuShares
		cpuPeriod int64 = defaultCpuPeriod
	)
	if hostCfg.CPUShares > 0 {
		cpuShares = hostCfg.CPUShares
	}
	if hostCfg.CPUPeriod > 0 {
		cpuPeriod = hostCfg.CPUPeriod
	}
	cpusetCount := countCPUSet(hostCfg.CpusetCpus)

	return event.Info{
		Container: event.Container{
			Type:           typeDocker.ToCTValue(),
			ID:             ctr.ID[:shortIDLength],
			Name:           name,
			Image:          cfg.Image,
			ImageDigest:    imageDigest,
			ImageID:        imageID,
			ImageRepo:      imageRepo,
			ImageTag:       imageTag,
			User:           cfg.User,
			CPUPeriod:      cpuPeriod,
			CPUQuota:       hostCfg.CPUQuota,
			CPUShares:      cpuShares,
			CPUSetCPUCount: cpusetCount,
			CreatedTime:    createdTime.Unix(),
			Env:            cfg.Env,
			FullID:         ctr.ID,
			HostIPC:        hostCfg.IpcMode.IsHost(),
			HostNetwork:    hostCfg.NetworkMode.IsHost(),
			HostPID:        hostCfg.PidMode.IsHost(),
			Ip:             ip,
			IsPodSandbox:   isPodSandbox,
			Labels:         labels,
			MemoryLimit:    hostCfg.Memory,
			SwapLimit:      hostCfg.MemorySwap,
			Privileged:     hostCfg.Privileged,
			PortMappings:   portMappings,
			Mounts:         mounts,
		},
	}
}

func (dc *dockerEngine) List(ctx context.Context) ([]event.Event, error) {
	containers, err := dc.ContainerList(ctx, container.ListOptions{All: true})
	if err != nil {
		return nil, err
	}

	evts := make([]event.Event, len(containers))
	for idx, ctr := range containers {
		ctrJson, err := dc.ContainerInspect(ctx, ctr.ID)
		if err != nil {
			// Minimum set of infos
			evts[idx] = event.Event{
				Info: event.Info{
					Container: event.Container{
						Type:        typeDocker.ToCTValue(),
						ID:          ctr.ID[:shortIDLength],
						Image:       ctr.Image,
						FullID:      ctr.ID,
						ImageID:     ctr.ImageID,
						CreatedTime: nanoSecondsToUnix(ctr.Created),
					},
				},
				IsCreate: true,
			}
		}
		evts[idx] = event.Event{
			IsCreate: true,
			Info:     dc.ctrToInfo(ctx, ctrJson),
		}
	}
	return evts, nil
}

func (dc *dockerEngine) Listen(ctx context.Context, wg *sync.WaitGroup) (<-chan event.Event, error) {
	outCh := make(chan event.Event)

	flts := filters.NewArgs()
	flts.Add("type", string(events.ContainerEventType))
	flts.Add("event", string(events.ActionCreate))
	flts.Add("event", string(events.ActionDestroy))
	msgs, _ := dc.Events(ctx, events.ListOptions{Filters: flts})
	wg.Add(1)
	go func() {
		defer close(outCh)
		defer wg.Done()
		for {
			select {
			case <-ctx.Done():
				return
			case msg := <-msgs:
				err := errors.New("inspect useless on action destroy")
				ctrJson := types.ContainerJSON{}
				if msg.Action == events.ActionCreate {
					ctrJson, err = dc.ContainerInspect(ctx, msg.Actor.ID)
				}
				if err != nil {
					// At least send an event with the minimum set of data
					outCh <- event.Event{
						Info: event.Info{
							Container: event.Container{
								Type:   typeDocker.ToCTValue(),
								ID:     msg.Actor.ID[:shortIDLength],
								FullID: msg.Actor.ID,
								Image:  msg.Actor.Attributes["image"],
							},
						},
						IsCreate: msg.Action == events.ActionCreate,
					}
				} else {
					outCh <- event.Event{
						Info:     dc.ctrToInfo(ctx, ctrJson),
						IsCreate: msg.Action == events.ActionCreate,
					}
				}
			}
		}
	}()
	return outCh, nil
}
