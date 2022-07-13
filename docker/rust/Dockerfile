FROM rust:1.62-slim AS build

RUN useradd --create-home --user-group nats
USER nats
WORKDIR /home/nats/code

COPY . ./
RUN cargo install --path .

FROM alpine

RUN adduser nats
USER nats

COPY --from=build /usr/local/cargo/bin/app /home/nats/

CMD ["/home/nats/app"]
