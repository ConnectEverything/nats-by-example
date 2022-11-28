FROM rust:1.65.0-slim AS build

WORKDIR /opt/app

COPY . ./

RUN cargo build

FROM debian:bullseye-slim

COPY --from=build /opt/app/target/debug/app /app

CMD ["/app"]
