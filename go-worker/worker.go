package main

// #include <stdbool.h>
// typedef void (*async_cb)(const char *json, bool added);
// extern void makeCallback(const char *json, bool added, async_cb cb) {
//     cb(json, added);
// }
import "C"

import (
    "context"
)

type asyncCb func(string, bool)

func workerLoop(ctx context.Context, cb asyncCb) {
    for {
        select {
        case <-ctx.Done():
          return
        }
    }
    cb("test", true)
}

func main() {
    // Noop, required to make CGO happy: `-buildmode=c-*` requires exactly one
    // main package, which in turn needs a `main` function`.
}