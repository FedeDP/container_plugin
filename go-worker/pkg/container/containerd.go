package container

import (
	"context"
	"github.com/containerd/containerd/api/events"
	containerd "github.com/containerd/containerd/v2/client"
	"github.com/containerd/containerd/v2/core/containers"
	"github.com/containerd/containerd/v2/pkg/namespaces"
	"github.com/containerd/containerd/v2/pkg/oci"
	"github.com/containerd/typeurl/v2"
	"github.com/opencontainers/runtime-spec/specs-go"
)

const typeContainerd Type = "containerd"

func init() {
	EngineGenerators[typeContainerd] = newContainerdEngine
}

type containerdEngine struct {
	client *containerd.Client
}

func newContainerdEngine(_ context.Context, socket string) (Engine, error) {
	client, err := containerd.New(socket)
	if err != nil {
		return nil, err
	}
	return &containerdEngine{client: client}, nil
}

func (c *containerdEngine) ctrToInfo(namespacedContext context.Context, container containerd.Container) Info {
	// https://github.com/falcosecurity/libs/blob/master/userspace/libsinsp/cri.hpp#L804
	info, err := container.Info(namespacedContext)
	if err != nil {
		info = containers.Container{}
	}
	spec, err := container.Spec(namespacedContext)
	if err != nil {
		spec = &oci.Spec{
			Process: &specs.Process{NoNewPrivileges: true},
			Mounts:  nil,
		}
	}

	// Cpu related
	var (
		cpuPeriod   uint64
		cpuQuota    int64
		cpuShares   uint64
		cpusetCount int64
	)
	if spec.Linux != nil && spec.Linux.Resources != nil && spec.Linux.Resources.CPU != nil {
		if spec.Linux.Resources.CPU.Period != nil {
			cpuPeriod = *spec.Linux.Resources.CPU.Period
		}
		if spec.Linux.Resources.CPU.Quota != nil {
			cpuQuota = *spec.Linux.Resources.CPU.Quota
		}
		if spec.Linux.Resources.CPU.Shares != nil {
			cpuShares = *spec.Linux.Resources.CPU.Shares
		}
		cpusetCount = countCPUSet(spec.Linux.Resources.CPU.Cpus)
	}

	// Mem related
	var (
		memoryLimit int64
		swapLimit   int64
	)
	if spec.Linux != nil && spec.Linux.Resources != nil && spec.Linux.Resources.Memory != nil {
		if spec.Linux.Resources.Memory.Limit != nil {
			memoryLimit = *spec.Linux.Resources.Memory.Limit
		}
		if spec.Linux.Resources.Memory.Swap != nil {
			swapLimit = *spec.Linux.Resources.Memory.Swap
		}
	}

	// Mounts related
	mounts := make([]Mount, 0)
	for _, m := range spec.Mounts {
		mounts = append(mounts, Mount{
			Source:      m.Source,
			Destination: m.Destination,
			Mode:        "",
			//RW:          m.ReadOnly, TODO
			//Propagation: string(m.Propagation), TODO
		})
	}

	// Namespace related
	var (
		hostIPC     bool
		hostPID     bool
		hostNetwork bool
	)

	for _, ns := range spec.Linux.Namespaces {
		if ns.Type == specs.PIDNamespace {
			hostPID = ns.Path == "host"
		}
		if ns.Type == specs.NetworkNamespace {
			hostNetwork = ns.Path == "host"
		}
		if ns.Type == specs.IPCNamespace {
			hostIPC = ns.Path == "host"
		}
	}

	// Image related
	// TODO https://github.com/falcosecurity/libs/blob/master/userspace/libsinsp/cri.hpp#L320

	// Network related
	// TODO https://github.com/falcosecurity/libs/blob/master/userspace/libsinsp/cri.hpp#L634C33-L634C62

	labels := make(map[string]string)
	for key, val := range info.Labels {
		if len(val) <= maxLabelLength {
			labels[key] = val
		}
	}
	return Info{
		Type:             string(typeContainerd),
		ID:               container.ID()[:12],
		Name:             "", //  // TODO container.m_name = status.metadata().name(); ??
		Image:            info.Image,
		ImageDigest:      "", // TODO
		ImageID:          "", // TODO
		ImageRepo:        "", // TODO
		ImageTag:         "", // TODO
		User:             spec.Process.User.Username,
		CniJson:          "", // TODO
		CPUPeriod:        int64(cpuPeriod),
		CPUQuota:         cpuQuota,
		CPUShares:        int64(cpuShares),
		CPUSetCPUCount:   cpusetCount,
		CreatedTime:      info.CreatedAt.Unix(),
		Env:              spec.Process.Env,
		FullID:           container.ID(),
		HostIPC:          hostIPC,
		HostNetwork:      hostNetwork,
		HostPID:          hostPID,
		Ip:               "",    // TODO
		IsPodSandbox:     false, // TODO
		Labels:           labels,
		MemoryLimit:      memoryLimit,
		SwapLimit:        swapLimit,
		PodSandboxID:     info.SandboxID,
		Privileged:       !spec.Process.NoNewPrivileges,
		PodSandboxLabels: nil, // TODO
		PortMappings:     nil, // TODO
		Mounts:           mounts,
	}
}

func (c *containerdEngine) List(ctx context.Context) ([]Event, error) {
	namespacesList, err := c.client.NamespaceService().List(ctx)
	if err != nil {
		return nil, err
	}
	evts := make([]Event, 0)
	for _, namespace := range namespacesList {
		namespacedContext := namespaces.WithNamespace(ctx, namespace)
		containersList, err := c.client.Containers(namespacedContext)
		if err != nil {
			continue
		}
		for _, container := range containersList {
			evts = append(evts, Event{
				Info:     c.ctrToInfo(namespacedContext, container),
				IsCreate: true,
			})
		}
	}
	return evts, nil
}

func (c *containerdEngine) Listen(ctx context.Context) (<-chan Event, error) {
	outCh := make(chan Event)
	eventsClient := c.client.EventService()
	eventsCh, _ := eventsClient.Subscribe(ctx,
		`topic=="/containers/create"`, `topic=="/containers/delete"`)
	go func() {
		defer close(outCh)
		for ev := range eventsCh {
			var (
				id       string
				isCreate bool
				image    string
				info     Info
			)
			ctrCreate := events.ContainerCreate{}
			err := typeurl.UnmarshalTo(ev.Event, &ctrCreate)
			if err == nil {
				id = ctrCreate.ID
				isCreate = true
				image = ctrCreate.Image
			} else {
				ctrDelete := events.ContainerDelete{}
				err = typeurl.UnmarshalTo(ev.Event, &ctrDelete)
				if err == nil {
					id = ctrDelete.ID
					isCreate = false
					image = ""
				}
			}
			namespacedContext := namespaces.WithNamespace(ctx, ev.Namespace)
			container, err := c.client.LoadContainer(namespacedContext, id)
			if err != nil {
				// minimum set of infos
				info = Info{
					Type:  string(typeContainerd),
					ID:    id,
					Image: image,
				}
			} else {
				info = c.ctrToInfo(namespacedContext, container)
			}
			outCh <- Event{
				Info:     info,
				IsCreate: isCreate,
			}
		}
	}()
	return outCh, nil
}
