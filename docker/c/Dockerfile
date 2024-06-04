FROM ubuntu:latest AS build

WORKDIR /opt/app

RUN apt-get update
RUN apt-get install -y build-essential
RUN apt-get install -y cmake
RUN apt-get install -y curl
RUN apt-get install -y git
RUN apt-get install -y golang
RUN apt-get install -y jq
RUN apt-get install -y libprotobuf-c-dev
RUN apt-get install -y libsodium-dev
RUN apt-get install -y libssl-dev
RUN apt-get install -y wget

COPY install-nats-c.sh .
COPY CMakeLists.txt .
RUN bash install-nats-c.sh

COPY . ./
RUN mkdir build && cd build && cmake .. && make

CMD ["/opt/app/build/app"]
