package main

import "github.com/FedeDP/container-worker/pkg/container"

/*
#include <stdbool.h>
#include <stdlib.h>
typedef void (*async_cb)(const char *json, bool added);
extern void makeCallback(const char *json, bool added, async_cb cb) {
	cb(json, added);
	free((void *)json);
}
*/
import "C"

import (
	"context"
	"github.com/FedeDP/container-worker/pkg/container/clients"
)

type asyncCb func(string, bool)

func workerLoop(ctx context.Context, cb asyncCb, listeners []clients.Listener) {
	for {
		select {
		case <-listeners[container.CtDocker]:
		case <-listeners[container.CtLxc]:
		case <-listeners[container.CtLibvirtLxc]:
		case <-listeners[container.CtMesos]:
		case <-listeners[container.CtCri]:
		case <-listeners[container.CtContainerd]:
		case <-listeners[container.CtCrio]:
		case <-listeners[container.CtBpm]:
		case <-listeners[container.CtPodman]:
			cb("json here", true)
			break
		case <-ctx.Done():
			return
		}
	}
}

func main() {
	// Noop, required to make CGO happy: `-buildmode=c-*` requires exactly one
	// main package, which in turn needs a `main` function`.
}
