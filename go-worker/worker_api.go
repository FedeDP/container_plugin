package main

import "C"
import (
	"encoding/json"
	"github.com/FedeDP/container-worker/pkg/container"
	"sync"
)

/*
#include <stdbool.h>
typedef const char cchar_t;
typedef void (*async_cb)(const char *json, bool added);
void makeCallback(const char *json, bool added, async_cb cb);
*/
import "C"

import (
	"context"
)

var (
	wg        sync.WaitGroup
	ctxCancel context.CancelFunc
)

//export StartWorker
func StartWorker(cb C.async_cb, initCfg *C.cchar_t) bool {
	var ctx context.Context
	ctx, ctxCancel = context.WithCancel(context.Background())

	var cfg map[string]container.SocketsEngine
	err := json.Unmarshal([]byte(C.GoString(initCfg)), &cfg)
	if err != nil {
		return false
	}

	// See https://github.com/enobufs/go-calls-c-pointer/blob/master/counter_api.go
	goCb := func(containerJson string, added bool) {
		if containerJson == "" {
			return
		}
		// Go cannot call C-function pointers.. Instead, use
		// a C-function to have it call the function pointer.
		cstr := C.CString(containerJson)
		cbool := C.bool(added)
		C.makeCallback(cstr, cbool, cb)
	}

	containerEngines := make([]container.Engine, 0)
	for engineName, engineGen := range container.EngineGenerators {
		engineCfg, ok := cfg[string(engineName)]
		if !ok || !engineCfg.Enabled {
			continue
		}
		// For each specified socket, start an engine
		for _, socket := range engineCfg.Sockets {
			engine, err := engineGen(ctx, socket)
			if err != nil {
				continue
			}
			containerEngines = append(containerEngines, engine)
			// List all pre-existing containers and run `goCb` on all of them
			containers, err := engine.List(ctx)
			if err == nil {
				for _, ctr := range containers {
					goCb(ctr.String(), true)
				}
			}
		}
	}
	if len(containerEngines) == 0 {
		return false
	}

	// Start worker goroutine
	wg.Add(1)
	go func() {
		defer wg.Done()
		workerLoop(ctx, goCb, containerEngines)
	}()
	return true
}

//export StopWorker
func StopWorker() {
	ctxCancel()
	wg.Wait()
}
