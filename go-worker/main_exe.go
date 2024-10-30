//go:build exe

package main

/*
#include <stdio.h>
#include <stdbool.h>
void echo_cb(const char *json, bool added) {
	printf("Added: %d, Json: %s\n", added, json);
}
*/
import "C"

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	fmt.Println("Starting worker")
	StartWorker((*[0]byte)(C.echo_cb))

	done := make(chan os.Signal, 1)
	signal.Notify(done, syscall.SIGINT, syscall.SIGTERM)
	fmt.Println("Blocking, press ctrl+c to continue...")
	<-done

	fmt.Println("Stopping worker")
	StopWorker()
}
