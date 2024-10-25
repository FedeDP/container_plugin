package main

// typedef void (*async_cb)(const char *json);
// extern void makeCallback(const char *json, async_cb cb) {
//     cb(json);
// }
import "C"

import (
    "context"
)

type asyncCb func(string)

func workerLoop(ctx context.Context, cb asyncCb) {
    for {
        select {
        case <-ctx.Done():
          return
        }
    }
    cb("test")
}

func main() {
    // Noop, required to make CGO happy: `-buildmode=c-*` requires exactly one
    // main package, which in turn needs a `main` function`.
}