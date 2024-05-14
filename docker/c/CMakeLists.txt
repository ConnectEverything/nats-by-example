cmake_minimum_required(VERSION 3.28)

set(CMAKE_BUILD_WITH_INSTALL_RPATH TRUE)
set(CMAKE_INSTALL_RPATH "/usr/local/lib")

file(GLOB SOURCES "*.c")
add_executable(app ${SOURCES})
target_link_libraries(app nats)
