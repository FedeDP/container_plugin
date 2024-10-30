package main

import "C"
import (
	"fmt"
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
		if containerJson == "" {
			return
		}
		if cb != nil {
			// Go cannot call C-function pointers.. Instead, use
			// a C-function to have it call the function pointer.
			cstr := C.CString(containerJson)
			cbool := C.bool(added)
			C.makeCallback(cstr, cbool, cb)
		} else {
			fmt.Println(containerJson, "added:", added)
		}
	}

	containerClients := make([]clients.Client, container.CtPodman+1)
	for i := container.CtDocker; i <= container.CtPodman; i++ {
		cl, err := i.ToClient(ctx)
		if err != nil {
			continue
		}
		containerClients[i] = cl
		// List all pre-existing containers and run `goCb` on all of them
		containers, err := cl.List(ctx)
		if err == nil {
			for _, ctr := range containers {
				goCb(ctr.String(), true)
			}
		}
	}

	// Start worker goroutine
	wg.Add(1)
	go func() {
		defer wg.Done()
		workerLoop(ctx, goCb, containerClients)
	}()
}

//export StopWorker
func StopWorker() {
	ctxCancel()
	wg.Wait()
}
