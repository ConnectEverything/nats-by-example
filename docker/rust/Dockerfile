FROM rust:1.62-slim AS build

WORKDIR /opt/app
COPY . ./
RUN cargo install --path .

FROM alpine
COPY --from=build /usr/local/cargo/bin/app /app
CMD ["/app"]
