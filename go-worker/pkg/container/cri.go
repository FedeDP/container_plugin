package container

import (
	"context"
	"github.com/FedeDP/container-worker/pkg/config"
	"github.com/FedeDP/container-worker/pkg/event"
	internalapi "k8s.io/cri-api/pkg/apis"
	v1 "k8s.io/cri-api/pkg/apis/runtime/v1"
	remote "k8s.io/cri-client/pkg"
	"strings"
	"sync"
	"time"
)

const (
	typeCri  engineType = "cri"
	typeCrio engineType = "cri-o"
)

func init() {
	engineGenerators[typeCri] = newCriEngine
}

type criEngine struct {
	client  internalapi.RuntimeService
	runtime int // as CT_FOO value
}

// See https://github.com/falcosecurity/libs/blob/4d04cad02cd27e53cb18f431361a4d031836bb75/userspace/libsinsp/cri.hpp#L71
func getRuntime(runtime string) int {
	if runtime == "containerd" || runtime == "cri-o" {
		tp := engineType(runtime)
		return tp.ToCTValue()
	}
	return typeCri.ToCTValue()
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

func (c *criEngine) ctrToInfo(ctr *v1.ContainerStatus, podSandboxStatus *v1.PodSandboxStatus,
	_ map[string]string) event.Info {

	// TODO parse info["info"] as json -> https://github.com/falcosecurity/libs/blob/master/userspace/libsinsp/cri.hpp#L263
	//
	// then parse env/privileged/image infos https://github.com/falcosecurity/libs/blob/master/userspace/libsinsp/cri.hpp#L481

	user := ctr.GetUser()
	if user == nil {
		user = &v1.ContainerUser{}
	}

	// Cpu related
	var (
		cpuPeriod   int64 = defaultCpuPeriod
		cpuQuota    int64
		cpuShares   int64 = defaultCpuShares
		cpusetCount int64
	)
	// Memory related
	var (
		memoryLimit int64
		swapLimit   int64
	)
	if ctr.GetResources().GetLinux() != nil {
		if ctr.GetResources().GetLinux().CpuPeriod > 0 {
			cpuPeriod = ctr.GetResources().GetLinux().CpuPeriod
		}
		cpuQuota = ctr.GetResources().GetLinux().CpuQuota
		if ctr.GetResources().GetLinux().CpuShares > 0 {
			cpuShares = ctr.GetResources().GetLinux().CpuShares
		}
		cpusetCount = countCPUSet(ctr.GetResources().GetLinux().CpusetCpus)

		memoryLimit = ctr.GetResources().GetLinux().MemoryLimitInBytes
		swapLimit = ctr.GetResources().GetLinux().MemorySwapLimitInBytes
	}

	mounts := make([]event.Mount, 0)
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
		mounts = append(mounts, event.Mount{
			Source:      m.HostPath,
			Destination: m.ContainerPath,
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
		if len(val) <= config.GetLabelMaxLen() {
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
		if len(val) <= config.GetLabelMaxLen() {
			podSandboxLabels[key] = val
		}
	}

	var size int64 = -1
	if config.GetWithSize() {
		stats, _ := c.client.ContainerStats(context.TODO(), ctr.Id)
		if stats != nil {
			size = int64(stats.GetWritableLayer().GetUsedBytes().GetValue())
		}
	}

	var (
		imageRepo string
		imageTag  string
	)
	imageRepoTag := strings.Split(ctr.GetImage().GetImage(), ":")
	if len(imageRepoTag) == 2 {
		imageRepo = imageRepoTag[0]
		imageTag = imageRepoTag[1]
	}

	return event.Info{
		Container: event.Container{
			Type:             c.runtime,
			ID:               ctr.Id[:shortIDLength],
			Name:             ctr.GetMetadata().GetName(),
			Image:            ctr.GetImage().GetImage(),
			ImageDigest:      ctr.ImageRef,
			ImageID:          ctr.ImageId,
			ImageRepo:        imageRepo,
			ImageTag:         imageTag,
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
			Mounts:           mounts,
			Size:             size,
		},
	}
}

func (c *criEngine) List(ctx context.Context) ([]event.Event, error) {
	ctrs, err := c.client.ListContainers(ctx, &v1.ContainerFilter{State: &v1.ContainerStateValue{}})
	if err != nil {
		return nil, err
	}
	evts := make([]event.Event, len(ctrs))
	for idx, ctr := range ctrs {
		container, err := c.client.ContainerStatus(ctx, ctr.Id, false)
		if err != nil || container.Status == nil {
			evts[idx] = event.Event{
				IsCreate: true,
				Info: event.Info{
					Container: event.Container{
						Type:        c.runtime,
						ID:          ctr.Id[:shortIDLength],
						FullID:      ctr.Id,
						ImageID:     ctr.ImageId,
						CreatedTime: nanoSecondsToUnix(ctr.CreatedAt),
						Labels:      ctr.Labels,
					},
				},
			}
		} else {
			podSandboxStatus, _ := c.client.PodSandboxStatus(ctx, ctr.PodSandboxId, false)
			if podSandboxStatus == nil {
				podSandboxStatus = &v1.PodSandboxStatusResponse{}
			}
			evts[idx] = event.Event{
				IsCreate: true,
				Info:     c.ctrToInfo(container.Status, podSandboxStatus.Status, container.Info),
			}
		}
	}
	return evts, nil
}

func (c *criEngine) Listen(ctx context.Context, wg *sync.WaitGroup) (<-chan event.Event, error) {
	containerEventsCh := make(chan *v1.ContainerEventResponse)
	wg.Add(1)
	go func() {
		defer close(containerEventsCh)
		defer wg.Done()
		_ = c.client.GetContainerEvents(ctx, containerEventsCh, nil)
	}()
	outCh := make(chan event.Event)
	wg.Add(1)
	go func() {
		defer close(outCh)
		defer wg.Done()
		for {
			select {
			case <-ctx.Done():
				return
			case evt := <-containerEventsCh:
				if evt.ContainerEventType == v1.ContainerEventType_CONTAINER_CREATED_EVENT ||
					evt.ContainerEventType == v1.ContainerEventType_CONTAINER_DELETED_EVENT {

					var info event.Info
					ctr, err := c.client.ContainerStatus(ctx, evt.ContainerId, false)
					if err != nil || ctr == nil {
						info = event.Info{
							Container: event.Container{
								Type:        c.runtime,
								ID:          evt.ContainerId[:shortIDLength],
								FullID:      evt.ContainerId,
								CreatedTime: nanoSecondsToUnix(evt.CreatedAt),
							},
						}
					} else {
						cPodSandbox := evt.GetPodSandboxStatus()
						info = c.ctrToInfo(ctr.Status, cPodSandbox, ctr.Info)
					}
					outCh <- event.Event{
						Info:     info,
						IsCreate: evt.ContainerEventType == v1.ContainerEventType_CONTAINER_CREATED_EVENT,
					}
				}
			}
		}
	}()
	return outCh, nil
}
