package container

import (
	"context"
	"github.com/FedeDP/container-worker/pkg/config"
	"github.com/FedeDP/container-worker/pkg/event"
	"github.com/containerd/containerd/api/events"
	containerd "github.com/containerd/containerd/v2/client"
	"github.com/containerd/containerd/v2/core/containers"
	"github.com/containerd/containerd/v2/pkg/namespaces"
	"github.com/containerd/containerd/v2/pkg/oci"
	"github.com/containerd/typeurl/v2"
	"github.com/opencontainers/runtime-spec/specs-go"
	"strings"
	"sync"
)

const typeContainerd engineType = "containerd"

func init() {
	engineGenerators[typeContainerd] = newContainerdEngine
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

func (c *containerdEngine) ctrToInfo(namespacedContext context.Context, container containerd.Container) event.Info {
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
		cpuPeriod   uint64 = defaultCpuPeriod
		cpuQuota    int64
		cpuShares   uint64 = defaultCpuShares
		cpusetCount int64
	)
	if spec.Linux != nil && spec.Linux.Resources != nil && spec.Linux.Resources.CPU != nil {
		if spec.Linux.Resources.CPU.Period != nil && *spec.Linux.Resources.CPU.Period > 0 {
			cpuPeriod = *spec.Linux.Resources.CPU.Period
		}
		if spec.Linux.Resources.CPU.Quota != nil {
			cpuQuota = *spec.Linux.Resources.CPU.Quota
		}
		if spec.Linux.Resources.CPU.Shares != nil && *spec.Linux.Resources.CPU.Shares > 0 {
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

	// Mounts related - TODO double check
	mounts := make([]event.Mount, 0)
	for _, m := range spec.Mounts {
		readOnly := false
		for _, path := range spec.Linux.ReadonlyPaths {
			if path == m.Destination {
				readOnly = true
				break
			}
		}
		mounts = append(mounts, event.Mount{
			Source:      m.Source,
			Destination: m.Destination,
			RW:          !readOnly,
			Propagation: spec.Linux.RootfsPropagation,
		})
	}

	// Namespace related - FIXME
	var (
		hostIPC     bool
		hostPID     bool
		hostNetwork bool
	)
	if spec.Linux != nil {
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
	}

	// Image related - TODO
	var size int64 = -1
	var (
		imageName   string
		imageDigest string
		imageRepo   string
		imageTag    string
	)
	image, _ := container.Image(context.TODO())
	if image != nil {
		imageName = image.Name()
		imgConfig, _ := image.Config(context.TODO())
		imageDigest = imgConfig.Digest.String()
		if config.GetWithSize() {
			size, _ = image.Size(context.TODO())
		}
	}
	imageRepoTag := strings.Split(info.Image, ":")
	if len(imageRepoTag) == 2 {
		imageRepo = imageRepoTag[0]
		imageTag = imageRepoTag[1]
	}

	// Network related - TODO

	labels := make(map[string]string)
	for key, val := range info.Labels {
		if len(val) <= config.GetLabelMaxLen() {
			labels[key] = val
		}
	}

	isPodSandbox := false
	var podSandboxLabels map[string]string
	sandbox, _ := c.client.LoadSandbox(namespacedContext, info.SandboxID)
	if sandbox != nil {
		isPodSandbox = true
		sandboxLabels, _ := sandbox.Labels(namespacedContext)
		if len(sandboxLabels) > 0 {
			podSandboxLabels = make(map[string]string)
			for key, val := range sandboxLabels {
				if len(val) <= config.GetLabelMaxLen() {
					podSandboxLabels[key] = val
				}
			}
		}
	}

	return event.Info{
		Container: event.Container{
			Type:             typeContainerd.ToCTValue(),
			ID:               container.ID()[:shortIDLength],
			Name:             container.ID()[:shortIDLength],
			Image:            info.Image,
			ImageDigest:      imageDigest, // FIXME, empty
			ImageID:          imageName,   // FIXME, empty
			ImageRepo:        imageRepo,
			ImageTag:         imageTag,
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
			Ip:               "", // TODO
			IsPodSandbox:     isPodSandbox,
			Labels:           labels,
			MemoryLimit:      memoryLimit,
			SwapLimit:        swapLimit,
			PodSandboxID:     info.SandboxID,
			Privileged:       false, // TODO implement
			PodSandboxLabels: podSandboxLabels,
			Mounts:           mounts,
			Size:             size,
		},
	}
}

func (c *containerdEngine) List(ctx context.Context) ([]event.Event, error) {
	namespacesList, err := c.client.NamespaceService().List(ctx)
	if err != nil {
		return nil, err
	}
	evts := make([]event.Event, 0)
	for _, namespace := range namespacesList {
		namespacedContext := namespaces.WithNamespace(ctx, namespace)
		containersList, err := c.client.Containers(namespacedContext)
		if err != nil {
			continue
		}
		for _, container := range containersList {
			evts = append(evts, event.Event{
				Info:     c.ctrToInfo(namespacedContext, container),
				IsCreate: true,
			})
		}
	}
	return evts, nil
}

func (c *containerdEngine) Listen(ctx context.Context, wg *sync.WaitGroup) (<-chan event.Event, error) {
	outCh := make(chan event.Event)
	eventsClient := c.client.EventService()
	eventsCh, _ := eventsClient.Subscribe(ctx,
		`topic=="/containers/create"`, `topic=="/containers/delete"`)
	wg.Add(1)
	go func() {
		defer close(outCh)
		defer wg.Done()
		for {
			select {
			case <-ctx.Done():
				return
			case ev := <-eventsCh:
				var (
					id       string
					isCreate bool
					image    string
					info     event.Info
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
					info = event.Info{
						Container: event.Container{
							Type:   typeContainerd.ToCTValue(),
							ID:     id[:shortIDLength],
							FullID: id,
							Image:  image,
						},
					}
				} else {
					info = c.ctrToInfo(namespacedContext, container)
				}
				outCh <- event.Event{
					Info:     info,
					IsCreate: isCreate,
				}
			}
		}
	}()
	return outCh, nil
}
