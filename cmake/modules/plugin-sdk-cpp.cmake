include(FetchContent)

message(
  STATUS
    "Fetching plugin-sdk-cpp at 'https://github.com/falcosecurity/plugin-sdk-cpp.git'"
)

FetchContent_Declare(
  plugin-sdk-cpp
  GIT_REPOSITORY https://github.com/falcosecurity/plugin-sdk-cpp.git
  GIT_TAG eb80c81ccbd010acb756c98b63d3551e807fb753) # HEAD of https://github.com/falcosecurity/plugin-sdk-cpp/pull/41

FetchContent_MakeAvailable(plugin-sdk-cpp)
set(PLUGIN_SDK_INCLUDE "${plugin-sdk-cpp_SOURCE_DIR}/include")
message(STATUS "Using plugin-sdk-cpp include at '${PLUGIN_SDK_INCLUDE}'")
