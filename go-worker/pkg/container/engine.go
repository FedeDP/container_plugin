package container

import "C"
import (
	"context"
	"encoding/json"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const shortIDLength = 12

type engineType string

// ToCTValue returns integer representation: CT_DOCKER,CT_PODMAN etc etc
// See src/container_type.h
func (t engineType) ToCTValue() int {
	switch t {
	case typeDocker:
		return 0
	case typePodman:
		return 11
	case typeCri:
		return 6
	case typeContainerd:
		return 7
	case typeCrio:
		return 8
	default:
		return 0xffff // unknown
	}
}

type engineGenerator func(context.Context, string) (Engine, error)
type EngineGenerator func(ctx context.Context) (Engine, error)

var (
	engineGenerators = make(map[engineType]engineGenerator)
	maxLabelLen      = 100 // default value
)

type socketsEngine struct {
	Enabled bool     `json:"enabled"`
	Sockets []string `json:"sockets"`
}

type engineCfg struct {
	SocketsEngines map[string]socketsEngine `json:"engines"`
	LabelMaxLen    int                      `json:"label_max_len"`
}

func Generators(initCfg string) ([]EngineGenerator, error) {
	var c engineCfg
	err := json.Unmarshal([]byte(initCfg), &c)
	if err != nil {
		return nil, err
	}
	maxLabelLen = c.LabelMaxLen

	generators := make([]EngineGenerator, 0)
	for engineName, engineGen := range engineGenerators {
		engineCfg, ok := c.SocketsEngines[string(engineName)]
		if !ok || !engineCfg.Enabled {
			continue
		}
		// For each specified socket, return a closure to generate its engine
		for _, socket := range engineCfg.Sockets {
			generators = append(generators, func(ctx context.Context) (Engine, error) {
				return engineGen(ctx, socket)
			})
		}
	}
	return generators, nil
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
type Container struct {
	Type             int               `json:"type"`
	ID               string            `json:"id"`
	Name             string            `json:"name"`
	Image            string            `json:"image"`
	ImageDigest      string            `json:"imagedigest"`
	ImageID          string            `json:"imageid"`
	ImageRepo        string            `json:"imagerepo"`
	ImageTag         string            `json:"imagetag"`
	User             string            `json:"User"`
	CniJson          string            `json:"cni_json"` // cri only
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
	PodSandboxID     string            `json:"pod_sandbox_id"` // cri only
	Privileged       bool              `json:"privileged"`
	PodSandboxLabels map[string]string `json:"pod_sandbox_labels"` // cri only
	PortMappings     []PortMapping     `json:"port_mappings"`
	Mounts           []Mount           `json:"Mounts"`
}

// Info struct wraps Container because we need the `container` struct in the json for backward compatibility.
// Format:
/*
{
  "container": {
    "type": 0,
    "id": "2400edb296c5",
    "name": "sharp_poincare",
    "image": "fedora:38",
    "imagedigest": "sha256:b9ff6f23cceb5bde20bb1f79b492b98d71ef7a7ae518ca1b15b26661a11e6a94",
    "imageid": "0ca0fed353fb77c247abada85aebc667fd1f5fa0b5f6ab1efb26867ba18f2f0a",
    "imagerepo": "fedora",
    "imagetag": "38",
    "User": "",
    "cni_json": "",
    "cpu_period": 0,
    "cpu_quota": 0,
    "cpu_shares": 0,
    "cpuset_cpu_count": 0,
    "created_time": 1730977803,
    "env": [
      "PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
      "DISTTAG=f38container",
      "FGC=f38",
      "FBR=f38"
    ],
    "full_id": "2400edb296c5d631fef083a30c680f71801b0409a9676ee546c084d0087d7c7d",
    "host_ipc": false,
    "host_network": false,
    "host_pid": false,
    "ip": "",
    "is_pod_sandbox": false,
    "labels": {
      "maintainer": "Clement Verna <cverna@fedoraproject.org>"
    },
    "memory_limit": 0,
    "swap_limit": 0,
    "pod_sandbox_id": "",
    "privileged": false,
    "pod_sandbox_labels": null,
    "port_mappings": [],
    "Mounts": [
      {
        "Source": "/home/federico",
        "Destination": "/home/federico",
        "Mode": "",
        "RW": true,
        "Propagation": "rprivate"
      }
    ]
  }
}
*/
type Info struct {
	Container `json:"container"`
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
