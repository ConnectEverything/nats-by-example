FROM rust:1.62-slim AS build

RUN useradd --create-home --user-group nats
USER nats
RUN mkdir /home/nats/code

WORKDIR /home/nats/code

COPY . ./
RUN cargo build

FROM debian:bullseye-slim

RUN useradd --create-home --user-group nats
USER nats

COPY --from=build /home/nats/code/target/debug/app /home/nats/

CMD ["/home/nats/app"]
