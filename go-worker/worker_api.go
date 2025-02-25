package main

import "C"
import (
	"github.com/FedeDP/container-worker/pkg/config"
	"github.com/FedeDP/container-worker/pkg/container"
	"sync"
	"unsafe"
)

/*
#include <stdbool.h>
typedef const char cchar_t;
typedef void (*async_cb)(const char *json, bool added, int async_id);
void makeCallback(const char *json, bool added, int async_id, async_cb cb);
*/
import "C"

import (
	"context"
	"github.com/falcosecurity/plugin-sdk-go/pkg/ptr"
)

var (
	wg           sync.WaitGroup
	ctxCancel    context.CancelFunc
	stringBuffer ptr.StringBuffer
)

//export StartWorker
func StartWorker(cb C.async_cb, initCfg *C.cchar_t, asyncID C.int) bool {
	var ctx context.Context
	ctx, ctxCancel = context.WithCancel(context.Background())

	// See https://github.com/enobufs/go-calls-c-pointer/blob/master/counter_api.go
	goCb := func(containerJson string, added bool) {
		if containerJson == "" {
			return
		}
		// Go cannot call C-function pointers. Instead, use
		// a C-function to have it call the function pointer.
		stringBuffer.Write(containerJson)
		cbool := C.bool(added)
		cStr := (*C.char)(stringBuffer.CharPtr())
		C.makeCallback(cStr, cbool, asyncID, cb)
	}

	err := config.Load(ptr.GoString(unsafe.Pointer(initCfg)))
	if err != nil {
		return false
	}

	generators, inotifier, err := container.Generators()
	if err != nil {
		return false
	}

	containerEngines := make([]container.Engine, 0)
	for _, generator := range generators {
		engine, err := generator(ctx)
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
	// Always append the dummy engine that is required to
	// be able to fetch container infos on the fly given other enabled engines.
	containerEngines = append(containerEngines, container.NewDummyEngine(containerEngines))

	// Start worker goroutine
	wg.Add(1)
	go func() {
		defer wg.Done()
		workerLoop(ctx, goCb, containerEngines, inotifier)
	}()
	return true
}

//export StopWorker
func StopWorker() {
	ctxCancel()
	wg.Wait()
	stringBuffer.Free()
}

//export AskForContainerInfo
func AskForContainerInfo(containerId *C.cchar_t) {
	containerID := ptr.GoString(unsafe.Pointer(containerId))
	ch := container.GetDummyChan()
	if ch != nil {
		ch <- containerID
	}
}
