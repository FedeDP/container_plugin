cmake_minimum_required(VERSION 3.22)

if(NOT DEFINED CMAKE_BUILD_TYPE OR CMAKE_BUILD_TYPE STREQUAL "")
    set(CMAKE_BUILD_TYPE "release")
endif()

list(APPEND CMAKE_MODULE_PATH "${CMAKE_CURRENT_SOURCE_DIR}/cmake/modules")

# From now on, we have VCPKG_ARCH variable
include(arch)

# compiler related configs
include(compiler)

# Vcpkg related configs
include(vcpkg)

# project metadata
project(
        container
        VERSION 0.1.0
        DESCRIPTION "Falco container metadata enrichment Plugin"
        LANGUAGES CXX)

# Include capabilities module.
# This also defines CAPS_SOURCES.
include(caps)

file(GLOB SOURCES src/*.cpp src/matchers/*.cpp)

# project target
add_library(container SHARED ${SOURCES} ${CAPS_SOURCES})
set_target_properties(container PROPERTIES CXX_EXTENSIONS OFF)

# dependencies
if(ENABLE_ASYNC)
    include(go-worker)
    add_dependencies(container go-worker)
endif()
include(plugin-sdk-cpp)

# vcpkg dependencies
find_package(re2 REQUIRED)
find_package(spdlog CONFIG REQUIRED)

# project compilation options
target_compile_options(container PRIVATE "-fPIC")
target_compile_options(container PRIVATE "-Wl,-z,relro,-z,now")
target_compile_options(container PRIVATE "-fstack-protector-strong")
# When compiling in Debug mode, this will define the DEBUG symbol for use in your code.
target_compile_options(container PUBLIC "$<$<CONFIG:DEBUG>:-DDEBUG>")
target_compile_features(container PUBLIC cxx_std_17)

# project includes
target_include_directories(container PRIVATE src/ ${PLUGIN_SDK_INCLUDE} ${PLUGIN_SDK_DEPS_INCLUDE} ${WORKER_INCLUDE})

# project linked libraries
target_link_libraries(container PRIVATE spdlog::spdlog_header_only re2::re2 ${WORKER_DEP} ${WORKER_LIB})

option(ENABLE_TESTS "Enable build of unit tests" ON)
if (ENABLE_TESTS)
    add_subdirectory(${CMAKE_SOURCE_DIR}/test/)
endif ()