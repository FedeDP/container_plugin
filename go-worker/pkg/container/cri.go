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
	EngineGenerators[typeCri] = newCriEngine
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

func newCriEngine(ctx context.Context, socket string) (Engine, error) {
	client, err := remote.NewRemoteRuntimeService(socket, 5*time.Second, nil, nil)
	if err != nil {
		return nil, err
	}
	version, err := client.Version(ctx, "")
	if err != nil {
		return nil, err
	}
	return &criEngine{
		client:  client,
		runtime: getRuntime(version.RuntimeName),
	}, nil
}

func (c *criEngine) ctrToInfo(ctr *v1.ContainerStatus, podSandboxStatus *v1.PodSandboxStatus) Info {
	image := ctr.GetImage()
	var img string
	if image != nil {
		img = image.Image
	}
	metadata := ctr.GetMetadata()
	if metadata == nil {
		metadata = &v1.ContainerMetadata{}
	}

	user := ctr.GetUser()
	if user == nil {
		user = &v1.ContainerUser{}
	}

	// Cpu related
	var (
		cpuPeriod   int64
		cpuQuota    int64
		cpuShares   int64
		cpusetCount int64
	)
	// Memory related
	var (
		memoryLimit int64
		swapLimit   int64
	)
	if ctr.GetResources() != nil && ctr.GetResources().GetLinux() != nil {
		cpuPeriod = ctr.GetResources().GetLinux().CpuPeriod
		cpuQuota = ctr.GetResources().GetLinux().CpuQuota
		cpuShares = ctr.GetResources().GetLinux().CpuShares
		cpusetCount = countCPUSet(ctr.GetResources().GetLinux().CpusetCpus)

		memoryLimit = ctr.GetResources().GetLinux().MemoryLimitInBytes
		swapLimit = ctr.GetResources().GetLinux().MemorySwapLimitInBytes
	}

	mounts := make([]Mount, 0)
	for _, m := range ctr.Mounts {
		var propagation string
		switch m.Propagation {
		case v1.MountPropagation_PROPAGATION_PRIVATE:
			propagation = "private"
		case v1.MountPropagation_PROPAGATION_HOST_TO_CONTAINER:
			propagation = "rslave"
		case v1.MountPropagation_PROPAGATION_BIDIRECTIONAL:
			propagation = "rshared"
		default:
			propagation = "unknown"
		}
		mounts = append(mounts, Mount{
			Source:      m.HostPath,
			Destination: m.ContainerPath,
			Mode:        "",
			RW:          !m.Readonly,
			Propagation: propagation,
		})
	}

	isPodSandbox := podSandboxStatus != nil
	podSandboxID := ctr.Id
	if podSandboxStatus == nil {
		podSandboxStatus = &v1.PodSandboxStatus{
			Network: &v1.PodSandboxNetworkStatus{},
			Linux: &v1.LinuxPodSandboxStatus{
				Namespaces: &v1.Namespace{
					Options: &v1.NamespaceOption{},
				},
			},
		}
	} else {
		podSandboxID = podSandboxStatus.Id
	}

	labels := make(map[string]string)
	for key, val := range ctr.Labels {
		if len(val) <= maxLabelLength {
			labels[key] = val
		}
	}
	labels["io.kubernetes.sandbox.id"] = podSandboxID
	if podSandboxStatus.Metadata != nil {
		labels["io.kubernetes.pod.uid"] = podSandboxStatus.Metadata.Uid
		labels["io.kubernetes.pod.name"] = podSandboxStatus.Metadata.Name
		labels["io.kubernetes.pod.namespace"] = podSandboxStatus.Metadata.Namespace
	}

	podSandboxLabels := make(map[string]string)
	for key, val := range podSandboxStatus.Labels {
		if len(val) <= maxLabelLength {
			podSandboxLabels[key] = val
		}
	}

	return Info{
		Type:             c.runtime,
		ID:               ctr.Id[:12],
		Name:             metadata.Name,
		Image:            img,
		ImageDigest:      ctr.ImageRef,
		ImageID:          ctr.ImageId,
		ImageRepo:        "", // TODO
		ImageTag:         "", // TODO
		User:             user.String(),
		CniJson:          "", // TODO
		CPUPeriod:        cpuPeriod,
		CPUQuota:         cpuQuota,
		CPUShares:        cpuShares,
		CPUSetCPUCount:   cpusetCount,
		CreatedTime:      nanoSecondsToUnix(ctr.CreatedAt),
		Env:              nil, // TODO
		FullID:           ctr.Id,
		HostIPC:          podSandboxStatus.Linux.Namespaces.Options.Ipc == v1.NamespaceMode_NODE,
		HostNetwork:      podSandboxStatus.Linux.Namespaces.Options.Network == v1.NamespaceMode_NODE,
		HostPID:          podSandboxStatus.Linux.Namespaces.Options.Pid == v1.NamespaceMode_NODE,
		Ip:               podSandboxStatus.Network.Ip,
		IsPodSandbox:     isPodSandbox,
		Labels:           labels,
		MemoryLimit:      memoryLimit,
		SwapLimit:        swapLimit,
		PodSandboxID:     podSandboxID,
		Privileged:       false, // TODO
		PodSandboxLabels: podSandboxLabels,
		PortMappings:     nil, // TODO
		Mounts:           mounts,
	}
}

func (c *criEngine) List(ctx context.Context) ([]Event, error) {
	ctrs, err := c.client.ListContainers(ctx, &v1.ContainerFilter{State: &v1.ContainerStateValue{}})
	if err != nil {
		return nil, err
	}
	evts := make([]Event, len(ctrs))
	for idx, ctr := range ctrs {
		status, err := c.client.ContainerStatus(ctx, ctr.Id, false)
		if err != nil || status.Status == nil {
			evts[idx] = Event{
				IsCreate: true,
				Info: Info{
					Type:        c.runtime,
					ID:          ctr.Id[:12],
					FullID:      ctr.Id,
					ImageID:     ctr.ImageId,
					CreatedTime: nanoSecondsToUnix(ctr.CreatedAt),
					Labels:      ctr.Labels,
				},
			}
		} else {
			podSandboxStatus, _ := c.client.PodSandboxStatus(ctx, ctr.PodSandboxId, false)
			if podSandboxStatus == nil {
				podSandboxStatus = &v1.PodSandboxStatusResponse{}
			}
			evts[idx] = Event{
				IsCreate: true,
				Info:     c.ctrToInfo(status.Status, podSandboxStatus.Status),
			}
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
				cPodSandbox := event.GetPodSandboxStatus()
				for _, ctr := range cStatus {
					var info Info
					if ctr == nil {
						// No pod sandbox infos
						var img string
						if ctr != nil && ctr.GetImage() != nil {
							img = ctr.GetImage().Image
						}
						info = Info{
							Type:        c.runtime,
							ID:          event.ContainerId,
							Image:       img,
							CreatedTime: nanoSecondsToUnix(event.CreatedAt),
						}
					} else {
						info = c.ctrToInfo(ctr, cPodSandbox)
					}
					outCh <- Event{
						Info:     info,
						IsCreate: event.ContainerEventType == v1.ContainerEventType_CONTAINER_CREATED_EVENT,
					}
				}
			}
		}
	}()
	return outCh, nil
}
