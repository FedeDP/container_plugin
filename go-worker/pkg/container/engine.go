package container

import (
	"context"
	"encoding/json"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type Type string
type EngineGenerator func(context.Context, string) (Engine, error)

var EngineGenerators = make(map[Type]EngineGenerator)

const maxLabelLength = 100

type SocketsEngine struct {
	Enabled bool     `json:"enabled"`
	Sockets []string `json:"sockets"`
}

type PortMapping struct {
	HostIp        string `json:"HostIp"`
	HostPort      string `json:"HostPort"`
	ContainerPort int    `json:"ContainerPort"`
}

type Mount struct {
	Source      string `json:"Source"`
	Destination string `json:"Destination"`
	Mode        string `json:"Mode"`
	RW          bool   `json:"RW"`
	Propagation string `json:"Propagation"`
}

// TODO add healtcheck/liveness/readiness probe related fields
type Info struct {
	Type             string            `json:"type"`
	ID               string            `json:"id"`
	Name             string            `json:"name"`
	Image            string            `json:"image"`
	ImageDigest      string            `json:"imagedigest"`
	ImageID          string            `json:"imageid"`
	ImageRepo        string            `json:"imagerepo"`
	ImageTag         string            `json:"image_tag"`
	User             string            `json:"user"`
	CniJson          string            `json:"cni_json"`
	CPUPeriod        int64             `json:"cpu_period"`
	CPUQuota         int64             `json:"cpu_quota"`
	CPUShares        int64             `json:"cpu_shares"`
	CPUSetCPUCount   int64             `json:"cpuset_cpu_count"`
	CreatedTime      int64             `json:"created_time"`
	Env              []string          `json:"env"`
	FullID           string            `json:"full_id"`
	HostIPC          bool              `json:"host_ipc"`
	HostNetwork      bool              `json:"host_network"`
	HostPID          bool              `json:"host_pid"`
	Ip               string            `json:"ip"`
	IsPodSandbox     bool              `json:"is_pod_sandbox"`
	Labels           map[string]string `json:"labels"`
	MemoryLimit      int64             `json:"memory_limit"`
	SwapLimit        int64             `json:"swap_limit"`
	MetadataDeadline uint64            `json:"metadata_deadline"`
	PodSandboxID     string            `json:"pod_sandbox_id"`
	Privileged       bool              `json:"privileged"`
	PodSandboxLabels map[string]string `json:"pod_sandbox_labels"`
	PortMappings     []PortMapping     `json:"port_mappings"`
	Mounts           []Mount           `json:"Mounts"`
}

type Event struct {
	Info
	IsCreate bool
}

func (i *Info) String() string {
	str, err := json.Marshal(i)
	if err != nil {
		return ""
	}
	return string(str)
}

type Engine interface {
	// List lists all running container for the engine
	List(ctx context.Context) ([]Event, error)
	// Listen returns a channel where container created/deleted events will be notified
	Listen(ctx context.Context) (<-chan Event, error)
}

func enforceUnixProtocolIfEmpty(socket string) string {
	base, _ := url.Parse(socket)
	if base.Scheme == "" {
		base.Scheme = "unix"
		return base.String()
	}
	return socket
}

func nanoSecondsToUnix(ns int64) int64 {
	return time.Unix(0, ns).Unix()
}

// Examples:
// 1,7 -> 2
// 1-4,7 -> 4 + 1 -> 5
// 1-4,7-10,12 -> 4 + 4 + 1 -> 9
func countCPUSet(cpuSet string) int64 {
	var counter int64
	cpusetParts := strings.Split(cpuSet, ",")
	for _, cpusetPart := range cpusetParts {
		cpuSetDash := strings.Split(cpusetPart, "-")
		if len(cpuSetDash) > 1 {
			if len(cpuSetDash) > 2 {
				// malformed
				return 0
			}
			start, err := strconv.ParseInt(cpuSetDash[0], 10, 64)
			if err != nil {
				return 0
			}
			end, err := strconv.ParseInt(cpuSetDash[1], 10, 64)
			if err != nil {
				return 0
			}
			counter += end - start + 1
		} else {
			counter++
		}
	}
	return counter
}
