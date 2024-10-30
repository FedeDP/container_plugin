package main

import (
	"github.com/FedeDP/container-worker/pkg/container"
	"github.com/FedeDP/container-worker/pkg/container/clients"
)

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
)

type asyncCb func(string, bool)

func workerLoop(ctx context.Context, cb asyncCb, containerClients []clients.Client) {
	var (
		evt clients.Event
		err error
	)

	channels := make([]<-chan clients.Event, len(containerClients))
	for i, client := range containerClients {
		if client != nil {
			channels[i], err = client.Listen(ctx)
			if err != nil {
				continue
			}
		}
	}

	for {
		select {
		case evt = <-channels[container.CtDocker]:
			cb(evt.String(), evt.IsCreate)
		case evt = <-channels[container.CtCri]:
			cb(evt.String(), evt.IsCreate)
		case evt = <-channels[container.CtContainerd]:
			cb(evt.String(), evt.IsCreate)
		case evt = <-channels[container.CtCrio]:
			cb(evt.String(), evt.IsCreate)
		case evt = <-channels[container.CtPodman]:
			cb(evt.String(), evt.IsCreate)
		case <-ctx.Done():
			return
		}
	}
}
