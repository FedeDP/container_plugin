package main

import "C"
import (
	"github.com/FedeDP/container-worker/pkg/container"
	"github.com/FedeDP/container-worker/pkg/container/clients"
	"sync"
)

/*
#include <stdbool.h>
typedef void (*async_cb)(const char *json, bool added);
void makeCallback(const char *json, bool added, async_cb cb);
*/
import "C"

import (
	"context"
	"encoding/json"
)

var (
	wg        sync.WaitGroup
	ctxCancel context.CancelFunc
)

//export StartWorker
func StartWorker(cb C.async_cb) {
	var ctx context.Context
	ctx, ctxCancel = context.WithCancel(context.Background())

	// See https://github.com/enobufs/go-calls-c-pointer/blob/master/counter_api.go
	goCb := func(containerJson string, added bool) {
		// Go cannot call C-function pointers.. Instead, use
		// a C-function to have it call the function pointer.
		cstr := C.CString(containerJson)
		cbool := C.bool(added)
		C.makeCallback(cstr, cbool, cb)
	}

	listeners := make([]clients.Listener, 0, container.CtPodman+1)
	for i := container.CtDocker; i <= container.CtPodman; i++ {
		cl, err := i.ToClient(ctx)
		if err != nil {
			continue
		}
		listener, err := cl.Listener(ctx)
		if err != nil {
			continue
		}
		listeners[i] = listener
		// List all pre-existing containers and run `goCb` on all of them
		containers, err := cl.List(ctx)
		if err == nil {
			for _, ctr := range containers {
				jsonStr, err := json.Marshal(ctr)
				if err != nil {
					continue
				}
				goCb(string(jsonStr), true)
			}
		}
	}

	// Start worker goroutine
	wg.Add(1)
	go func() {
		defer wg.Done()
		workerLoop(ctx, goCb, listeners)
	}()
}

//export StopWorker
func StopWorker() {
	ctxCancel()
	wg.Wait()
}
