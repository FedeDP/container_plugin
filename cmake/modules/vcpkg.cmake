execute_process(
        COMMAND uname -m
        COMMAND sed "s/x86_64/x64/"
        COMMAND sed "s/aarch64/arm64/"
        OUTPUT_VARIABLE ARCH_output
        ERROR_VARIABLE ARCH_error
        RESULT_VARIABLE ARCH_result
        OUTPUT_STRIP_TRAILING_WHITESPACE
)
if(${ARCH_result} EQUAL 0)
    set(ARCH ${ARCH_output})
    message(STATUS "Target arch: ${ARCH}")
else()
    message(
            FATAL_ERROR
            "Failed to determine target architecture: ${ARCH_error}"
    )
endif()

set(VCPKG_TARGET_TRIPLET "${ARCH}-linux-release")
set(CMAKE_TOOLCHAIN_FILE ${CMAKE_CURRENT_SOURCE_DIR}/vcpkg/scripts/buildsystems/vcpkg.cmake CACHE STRING "Vcpkg toolchain file")