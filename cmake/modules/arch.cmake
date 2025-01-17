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
    set(VCPKG_ARCH ${ARCH_output})
    message(STATUS "Target arch: ${VCPKG_ARCH}")
else()
    message(
            FATAL_ERROR
            "Failed to determine target architecture: ${ARCH_error}"
    )
endif()