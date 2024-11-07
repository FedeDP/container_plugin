cmake_minimum_required(VERSION 3.22)

list(APPEND CMAKE_MODULE_PATH "${CMAKE_CURRENT_SOURCE_DIR}/cmake/modules")

# project metadata
project(
  container
  VERSION 0.1.0
  DESCRIPTION "Falco container metadata enrichment Plugin"
  LANGUAGES CXX)

# dependencies
include(FetchContent)
include(go-worker)
include(spdlog)
include(plugin-sdk-cpp)

file(GLOB_RECURSE SOURCES src/*.cpp)

# project target
add_library(container SHARED ${SOURCES})
set_target_properties(container PROPERTIES CXX_EXTENSIONS OFF)

# project compilation options
target_compile_options(container PRIVATE "-fPIC")
target_compile_options(container PRIVATE "-Wl,-z,relro,-z,now")
target_compile_options(container PRIVATE "-fstack-protector-strong")
# When compiling in Debug mode, this will define the DEBUG symbol for use in
# your code.
target_compile_options(container PUBLIC "$<$<CONFIG:DEBUG>:-DDEBUG>")
target_compile_features(container PUBLIC cxx_std_17)

# TODO: use re2 not from system
find_package(PkgConfig)
pkg_check_modules(RE2 REQUIRED re2)

# project includes
target_include_directories(
        container PRIVATE ${PLUGIN_SDK_INCLUDE} ${SPDLOG_INCLUDE} ${RE2_INCLUDE_DIRS} ${WORKER_INCLUDE})

# project linked libraries
target_link_libraries(container PRIVATE ${RE2_LIBRARIES} ${WORKER_LIB})
