include(FetchContent)

message(
  STATUS
    "Fetching plugin-sdk-cpp at 'https://github.com/falcosecurity/plugin-sdk-cpp.git'"
)

FetchContent_Declare(
  plugin-sdk-cpp
  GIT_REPOSITORY https://github.com/falcosecurity/plugin-sdk-cpp.git
  GIT_TAG 5e1273e88f915081a2159971a5ec02b301a1526c) # HEAD of https://github.com/falcosecurity/plugin-sdk-cpp/pull/41

FetchContent_MakeAvailable(plugin-sdk-cpp)
set(PLUGIN_SDK_INCLUDE "${plugin-sdk-cpp_SOURCE_DIR}/include")
# Since we use nlohmann-json provided by the sdk-cpp plugin, we need to also use use this one
set(PLUGIN_SDK_DEPS_INCLUDE "${plugin-sdk-cpp_SOURCE_DIR}/include/falcosecurity/internal/deps")
message(STATUS "Using plugin-sdk-cpp include at '${PLUGIN_SDK_INCLUDE}'")
