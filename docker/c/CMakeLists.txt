cmake_minimum_required(VERSION 3.28)

set(CMAKE_BUILD_WITH_INSTALL_RPATH TRUE)
set(CMAKE_INSTALL_RPATH "/usr/local/lib")

file(GLOB SOURCES "*.c")
add_executable(app ${SOURCES})
target_link_libraries(app nats)

# Fail the build on API mismatches instead of crashing at runtime (passing
# the wrong pointer type is only a warning by default before GCC 14).
target_compile_options(app PRIVATE
  -Werror=incompatible-pointer-types
  -Werror=int-conversion
  -Werror=implicit-function-declaration)
