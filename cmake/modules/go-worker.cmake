include(ExternalProject)

message(STATUS "Building go-worker static library")

if(CMAKE_HOST_SYSTEM_NAME STREQUAL "Linux")
    # btrfs cmake dep
    include(btrfs)

    ## gpgme is vcpkg installed but comes with pkg-config
    ## It is a runtime dep of podman go package.
    find_package(PkgConfig REQUIRED)
    pkg_check_modules(GPGME REQUIRED gpgme IMPORTED_TARGET)

    # Pkg config paths to all gpgme libs (libgpgme, libgpg-error and libassuan)
    # to be passed down to go-worker to let it find the needed deps.
    set(VCPKG_PKGCONFIG_PATH "${CMAKE_SOURCE_DIR}/vcpkg/packages/gpgme_${VCPKG_ARCH}-linux-release/lib/pkgconfig/:${CMAKE_SOURCE_DIR}/vcpkg/packages/libgpg-error_${VCPKG_ARCH}-linux-release/lib/pkgconfig/:${CMAKE_SOURCE_DIR}/vcpkg/packages/libassuan_${VCPKG_ARCH}-linux-release/lib/pkgconfig/")
    set(WORKER_DEP PkgConfig::GPGME)
endif()

ExternalProject_Add(go-worker
        DEPENDS ${WORKER_DEP}
        SOURCE_DIR ${CMAKE_SOURCE_DIR}/go-worker
        BUILD_IN_SOURCE 1
        CONFIGURE_COMMAND ""
        BUILD_COMMAND make -e PKG_CONFIG_PATH=${VCPKG_PKGCONFIG_PATH} ${BTRFS_CGO_CFLAG} lib
        BUILD_BYPRODUCTS libworker.a libworker.h
        INSTALL_COMMAND ""
)

# https://tip.golang.org/doc/go1.20#cgo:
# > A consequence is that, on macOS, if Go code that uses the net package is built with -buildmode=c-archive,
# linking the resulting archive into a C program requires passing -lresolv when linking the C code.
# So, properly link resolv library; also, we need foundation library.
if(APPLE)
    find_library(SECURITY_FRAMEWORK Security REQUIRED)
    find_library(RESOLV resolv REQUIRED)
    set(WORKER_DEP ${SECURITY_FRAMEWORK} ${RESOLV})
endif()
set(WORKER_LIB ${CMAKE_SOURCE_DIR}/go-worker/libworker.a)
set(WORKER_INCLUDE ${CMAKE_SOURCE_DIR}/go-worker)

message(STATUS "Using worker library at '${WORKER_LIB}' with header in ${WORKER_INCLUDE}")