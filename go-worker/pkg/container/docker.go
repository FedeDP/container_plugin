package container

import (
	"context"
	"errors"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
	"strings"
)

const typeDocker Type = "docker"

func init() {
	EngineGenerators[typeDocker] = newDockerEngine
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

func (dc *dockerEngine) ctrToInfo(ctx context.Context, ctr types.ContainerJSON) Info {
	hostCfg := ctr.HostConfig
	if hostCfg == nil {
		hostCfg = &container.HostConfig{}
	}
	mounts := make([]Mount, 0)
	for _, m := range ctr.Mounts {
		mounts = append(mounts, Mount{
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
	portMappings := make([]PortMapping, 0)
	for port, portBindings := range netCfg.Ports {
		if port.Proto() != "tcp" {
			continue
		}
		containerPort := port.Int()
		for _, portBinding := range portBindings {
			portMappings = append(portMappings, PortMapping{
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

	return Info{
		Type:             string(typeDocker),
		ID:               ctr.ID[:12],
		Name:             name,
		Image:            cfg.Image,
		ImageDigest:      imageDigest,
		ImageID:          imageID,
		ImageRepo:        imageRepo,
		ImageTag:         imageTag,
		User:             cfg.User,
		CniJson:          "", // TODO
		CPUPeriod:        hostCfg.CPUPeriod,
		CPUQuota:         hostCfg.CPUQuota,
		CPUShares:        hostCfg.CPUShares,
		CPUSetCPUCount:   hostCfg.CPUCount,
		CreatedTime:      ctr.Created,
		Env:              cfg.Env,
		FullID:           ctr.ID,
		HostIPC:          hostCfg.IpcMode.IsHost(),
		HostNetwork:      hostCfg.NetworkMode.IsHost(),
		HostPID:          hostCfg.PidMode.IsHost(),
		Ip:               netCfg.IPAddress,
		IsPodSandbox:     isPodSandbox,
		Labels:           cfg.Labels,
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

func (dc *dockerEngine) List(ctx context.Context) ([]Event, error) {
	containers, err := dc.ContainerList(ctx, container.ListOptions{All: true})
	if err != nil {
		return nil, err
	}

	evts := make([]Event, len(containers))
	for idx, ctr := range containers {
		ctrJson, err := dc.ContainerInspect(ctx, ctr.ID)
		if err != nil {
			// Minimum set of infos
			evts[idx] = Event{
				Info: Info{
					Type:  string(typeDocker),
					ID:    ctr.ID,
					Image: ctr.Image,
				},
				IsCreate: true,
			}
		}
		evts[idx] = Event{
			IsCreate: true,
			Info:     dc.ctrToInfo(ctx, ctrJson),
		}
	}
	return evts, nil
}

func (dc *dockerEngine) Listen(ctx context.Context) (<-chan Event, error) {
	outCh := make(chan Event)

	flts := filters.NewArgs()
	flts.Add("type", string(events.ContainerEventType))
	flts.Add("event", string(events.ActionCreate))
	flts.Add("event", string(events.ActionDestroy))
	msgs, _ := dc.Events(ctx, events.ListOptions{Filters: flts})
	go func() {
		defer close(outCh)
		for msg := range msgs {
			err := errors.New("inspect useless on action destroy")
			ctrJson := types.ContainerJSON{}
			if msg.Action == events.ActionCreate {
				ctrJson, err = dc.ContainerInspect(ctx, msg.Actor.ID)
			}
			if err != nil {
				// At least send an event with the minimal set of data
				outCh <- Event{
					Info: Info{
						Type:  string(typeDocker),
						ID:    msg.Actor.ID,
						Image: msg.Actor.Attributes["image"],
					},
					IsCreate: msg.Action == events.ActionCreate,
				}
			} else {
				outCh <- Event{
					Info:     dc.ctrToInfo(ctx, ctrJson),
					IsCreate: msg.Action == events.ActionCreate,
				}
			}
		}
	}()
	return outCh, nil
}
