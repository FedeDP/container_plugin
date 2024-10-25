package main

// typedef void (*async_cb)(const char *json);
// void makeCallback(const char *json, cb async_cb);
import "C"

import (
    "sync"
    "context"
)

var (
    wg sync.WaitGroup
    ctxCancel context.Context
)

//export StartWorker
func StartWorker(cb C.async_cb) {
    var ctx context.Context
    ctx, ctxCancel = context.WithCancel(context.Background())

    // See https://github.com/enobufs/go-calls-c-pointer/blob/master/counter_api.go
    goCb := func(containerJson string) {
    	// Go cannot call C-function pointers.. Instead, use
    	// a C-function to have it call the function pointer.
    	cstr := C.CString(str)
   		C.makeCallback(cstr, cb)
   		C.free(unsafe.Pointer(cstr))
   	}

    // TODO: list all pre-existing containers and run `goCb` on all of them

    // Start worker goroutine
    wg.Add(1)
    go func() {
        defer wg.Done()
        workerLoop(ctx, goCb)
    }
}

//export StopWorker
func StopWorker() {
    ctxCancel()
    wg.Wait()
}