#pragma once

typedef void (*async_cb)(const char *json);

extern void StartWorker(async_cb cb);
extern void StopWorker();
