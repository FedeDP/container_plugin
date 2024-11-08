package main

import (
	"github.com/FedeDP/container-worker/pkg/container"
	"reflect"
)

/*
#include <stdbool.h>
#include <stdlib.h>
typedef void (*async_cb)(const char *json, bool added, int async_id);
extern void makeCallback(const char *json, bool added, int async_id, async_cb cb) {
	cb(json, added, async_id);
	free((void *)json);
}
*/
import "C"

import (
	"context"
)

type asyncCb func(string, bool)

func workerLoop(ctx context.Context, cb asyncCb, containerEngines []container.Engine) {
	var (
		evt container.Event
		err error
	)

	channels := make([]<-chan container.Event, len(containerEngines))
	// We need to use a reflect.SelectCase here since
	// we will need to select a variable number of channels
	cases := make([]reflect.SelectCase, 0)
	for i, engine := range containerEngines {
		channels[i], err = engine.Listen(ctx)
		if err != nil {
			continue
		}
		cases = append(cases, reflect.SelectCase{
			Dir:  reflect.SelectRecv,
			Chan: reflect.ValueOf(channels[i]),
		})
	}

	// Emplace back case for `ctx.Done` channel
	cases = append(cases, reflect.SelectCase{
		Dir:  reflect.SelectRecv,
		Chan: reflect.ValueOf(ctx.Done()),
	})

	for {
		chosen, val, _ := reflect.Select(cases)
		if chosen == len(cases)-1 {
			// ctx.Done!
			return
		} else {
			evt, _ = val.Interface().(container.Event)
			cb(evt.String(), evt.IsCreate)
		}
	}
}
