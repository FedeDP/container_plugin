set(CAPS_SOURCES "")

macro(ADD_CAP cap)
    option(ENABLE_${cap} "Enable support for ${cap} capability" ON)
    if(${ENABLE_${cap}})
        message(STATUS "${cap} capability enabled")
        add_compile_definitions(_HAS_${cap})
        string(TOLOWER ${cap} lower_cap)
        file(GLOB_RECURSE SOURCES src/caps/${lower_cap}/*.cpp)
        list(APPEND CAPS_SOURCES ${SOURCES})

        # Append to vcpkg manifest features - only useful for ASYNC cap
        list(APPEND VCPKG_MANIFEST_FEATURES ${lower_cap})
    endif()
endmacro()

ADD_CAP(ASYNC)
ADD_CAP(EXTRACT)
ADD_CAP(LISTENING)
ADD_CAP(PARSE)

if(NOT ENABLE_ASYNC AND NOT ENABLE_EXTRACT AND NOT ENABLE_LISTENING AND NOT ENABLE_PARSE)
    message(FATAL_ERROR "No capabilities enabled.")
endif ()