# Only build release versions of the ports:
# https://learn.microsoft.com/en-us/vcpkg/users/triplets#vcpkg_build_type
set(VCPKG_BUILD_TYPE release)
set(VCPKG_CRT_LINKAGE static)
set(VCPKG_LIBRARY_LINKAGE static)
set(CMAKE_TOOLCHAIN_FILE ${CMAKE_CURRENT_SOURCE_DIR}/vcpkg/scripts/buildsystems/vcpkg.cmake CACHE STRING "Vcpkg toolchain file")