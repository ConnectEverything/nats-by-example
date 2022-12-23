FROM rust:1.66-slim AS build

WORKDIR /opt/app

# Compile dependencies in one layer that can be reused across all examples.
COPY Cargo.toml .
RUN echo "fn main(){}" > main.rs && cargo build && rm main.rs

COPY . .

RUN cargo build

FROM debian:bullseye-slim

COPY --from=build /opt/app/target/debug/app /app

CMD ["/app"]
