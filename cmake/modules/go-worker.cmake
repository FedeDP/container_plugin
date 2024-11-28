include(ExternalProject)

message(STATUS "Building go-worker static library")

ExternalProject_Add(go-worker
        SOURCE_DIR ${CMAKE_SOURCE_DIR}/go-worker
        BUILD_IN_SOURCE 1
        CONFIGURE_COMMAND ""
        BUILD_COMMAND make lib
        BUILD_BYPRODUCTS libworker.a libworker.h
        INSTALL_COMMAND ""
)

set(WORKER_LIB ${CMAKE_SOURCE_DIR}/go-worker/libworker.a)
set(WORKER_INCLUDE ${CMAKE_SOURCE_DIR}/go-worker)

message(STATUS "Using worker library at '${WORKER_LIB}' with header in ${WORKER_INCLUDE}")

