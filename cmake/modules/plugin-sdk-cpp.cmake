message(
  STATUS
    "Fetching plugin-sdk-cpp at 'https://github.com/falcosecurity/plugin-sdk-cpp.git'"
)

FetchContent_Declare(
  plugin-sdk-cpp
  GIT_REPOSITORY https://github.com/falcosecurity/plugin-sdk-cpp.git
  GIT_TAG ecc5743c1d73f70cac32f92853bf70a5ff5b11ce) # HEAD of https://github.com/falcosecurity/plugin-sdk-cpp/pull/41

FetchContent_MakeAvailable(plugin-sdk-cpp)
set(PLUGIN_SDK_INCLUDE "${plugin-sdk-cpp_SOURCE_DIR}/include")
message(STATUS "Using plugin-sdk-cpp include at '${PLUGIN_SDK_INCLUDE}'")
