FROM rust:1.100-slim-trixie AS build

WORKDIR /opt/app

# Cache invalidation as Rust Cargo.toml refers main branch.
ARG CACHE_INVALIDATE="2022-12-29T13:26:05Z"

# Compile dependencies in one layer that can be reused across all examples.
COPY Cargo.toml .
RUN echo "fn main(){}" > main.rs && cargo build && rm main.rs && rm /opt/app/target/debug/app
COPY . .

RUN cargo update
# Touch main.rs: it is older than the dummy build above, so cargo would
# otherwise consider the binary up to date and relink the empty main().
RUN touch main.rs && cargo build

FROM debian:trixie-slim

COPY --from=build /opt/app/target/debug/app /app

CMD ["/app"]
