package main

import "C"
import (
	"github.com/FedeDP/container-worker/pkg/container"
	"sync"
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
)

var (
	wg        sync.WaitGroup
	ctxCancel context.CancelFunc
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
		// Go cannot call C-function pointers.. Instead, use
		// a C-function to have it call the function pointer.
		cstr := C.CString(containerJson)
		cbool := C.bool(added)
		C.makeCallback(cstr, cbool, asyncID, cb)
	}

	generators, err := container.Generators(C.GoString(initCfg))
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
