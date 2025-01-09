package container

import "C"
import (
	"context"
	"github.com/FedeDP/container-worker/pkg/config"
	"github.com/FedeDP/container-worker/pkg/event"
	"k8s.io/utils/inotify"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	shortIDLength = 12
	// Default values from
	//https://github.com/falcosecurity/libs/blob/39c0e0dcb9d1d23e46b13f4963a9a7106db1f650/userspace/libsinsp/container_info.h#L218
	defaultCpuPeriod = 100000
	defaultCpuShares = 1024
)

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

type EngineInotifier struct {
	watcher           *inotify.Watcher
	watcherGenerators map[string]EngineGenerator
}

func (e *EngineInotifier) Listen() <-chan *inotify.Event {
	if e.watcher == nil {
		return nil
	}
	return e.watcher.Event
}

func (e *EngineInotifier) Process(ctx context.Context, val interface{}) Engine {
	ev, _ := val.(*inotify.Event)
	if cb, ok := e.watcherGenerators[ev.Name]; ok {
		_ = e.watcher.RemoveWatch(filepath.Dir(ev.Name))
		engine, _ := cb(ctx)
		return engine
	} else {
		// If the new created path is a folder, check if
		// it is a subpath of any watcherGenerator socket,
		// and eventually add a new watch, removing the old one
		fileInfo, err := os.Stat(ev.Name)
		if err != nil || !fileInfo.IsDir() {
			return nil
		}
		for socket, cb := range e.watcherGenerators {
			if strings.HasPrefix(socket, ev.Name) {
				// Remove old watch
				_ = e.watcher.RemoveWatch(filepath.Dir(ev.Name))
				// It may happen that the actual socket has already been created.
				// Check it and if it is not created yet, add a new inotify watcher.
				if _, statErr := os.Stat(socket); os.IsNotExist(statErr) {
					// Add new watch
					_ = e.watcher.AddWatch(ev.Name, inotify.InCreate|inotify.InIsdir)
					break
				} else {
					engine, _ := cb(ctx)
					return engine
				}
			}
		}
	}
	return nil
}

func (e *EngineInotifier) Close() {
	if e.watcher == nil {
		return
	}
	_ = e.watcher.Close()
}

// Hooked up by each engine through init()
var engineGenerators = make(map[engineType]engineGenerator)

func Generators() ([]EngineGenerator, *EngineInotifier, error) {
	generators := make([]EngineGenerator, 0)
	engineNotifier := EngineInotifier{
		watcher:           nil,
		watcherGenerators: make(map[string]EngineGenerator),
	}

	c := config.Get()
	for engineName, engineGen := range engineGenerators {
		eCfg, ok := c.SocketsEngines[string(engineName)]
		if !ok || !eCfg.Enabled {
			continue
		}
		// For each specified socket, return a closure to generate its engine
		for _, socket := range eCfg.Sockets {
			if _, statErr := os.Stat(socket); os.IsNotExist(statErr) {
				// Does not exist; emplace back an inotify listener
				if engineNotifier.watcher == nil {
					engineNotifier.watcher, _ = inotify.NewWatcher()
				}
				if engineNotifier.watcher != nil {
					dir := filepath.Dir(socket)
					err := engineNotifier.watcher.AddWatch(dir, inotify.InCreate)
					if err != nil {
						// Try to attach watcher to parent dir
						// eg: /run/user for podman, /run/ for crio, and so on
						dir = filepath.Dir(dir)
						err = engineNotifier.watcher.AddWatch(dir, inotify.InCreate)
					}
					if err == nil {
						engineNotifier.watcherGenerators[socket] = func(ctx context.Context) (Engine, error) {
							return engineGen(ctx, socket)
						}
					}
				}
			} else {
				generators = append(generators, func(ctx context.Context) (Engine, error) {
					return engineGen(ctx, socket)
				})
			}
		}
	}
	return generators, &engineNotifier, nil
}

type Engine interface {
	// List lists all running container for the engine
	List(ctx context.Context) ([]event.Event, error)
	// Listen returns a channel where container created/deleted events will be notified
	Listen(ctx context.Context, wg *sync.WaitGroup) (<-chan event.Event, error)
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
	if cpuSet == "" {
		return counter
	}
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

func shortContainerID(id string) string {
	if len(id) > shortIDLength {
		return id[:shortIDLength]
	}
	return id
}
