message(STATUS "Finding go-worker static library")

find_library(WORKER_LIB worker REQUIRED HINTS ${CMAKE_CURRENT_BINARY_DIR})

message(STATUS "Using worker library at '${WORKER_LIB}'")