package main

import (
	"github.com/FedeDP/container-worker/pkg/container"
	"github.com/FedeDP/container-worker/pkg/event"
	"reflect"
	"sync"
)

/*
#include <stdbool.h>
#include <stdlib.h>
typedef void (*async_cb)(const char *json, bool added, int async_id);
extern void makeCallback(const char *json, bool added, int async_id, async_cb cb) {
	cb(json, added, async_id);
}
*/
import "C"

import (
	"context"
)

const (
	ctxDoneIdx   = 0
	inotifierIdx = 1
)

type asyncCb func(string, bool)

func workerLoop(ctx context.Context, cb asyncCb, containerEngines []container.Engine, inotifier *container.EngineInotifier, wg *sync.WaitGroup) {
	var evt event.Event

	// We need to use a reflect.SelectCase here since
	// we will need to select a variable number of channels
	cases := make([]reflect.SelectCase, 0)

	// Emplace back case for `ctx.Done` channel
	cases = append(cases, reflect.SelectCase{
		Dir:  reflect.SelectRecv,
		Chan: reflect.ValueOf(ctx.Done()),
	})

	// Emplace back case for inotifier channel if needed
	inotifierCh := inotifier.Listen()
	if inotifierCh != nil {
		cases = append(cases, reflect.SelectCase{
			Dir:  reflect.SelectRecv,
			Chan: reflect.ValueOf(inotifierCh),
		})
	}

	// Emplace back cases for each container engine listener
	for _, engine := range containerEngines {
		ch, err := engine.Listen(ctx, wg)
		if err != nil {
			continue
		}
		cases = append(cases, reflect.SelectCase{
			Dir:  reflect.SelectRecv,
			Chan: reflect.ValueOf(ch),
		})
	}

	for {
		chosen, val, _ := reflect.Select(cases)
		if chosen == ctxDoneIdx {
			// ctx.Done!
			break
		} else if inotifierCh != nil && chosen == inotifierIdx {
			// inotifier!
			engine := inotifier.Process(ctx, val.Interface())
			if engine != nil {
				ch, err := engine.Listen(ctx, wg)
				if err != nil {
					continue
				}
				cases = append(cases, reflect.SelectCase{
					Dir:  reflect.SelectRecv,
					Chan: reflect.ValueOf(ch),
				})
			}
		} else {
			evt, _ = val.Interface().(event.Event)
			cb(evt.String(), evt.IsCreate)
		}
	}

	inotifier.Close()
}
