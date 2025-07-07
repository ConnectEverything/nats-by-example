FROM crystallang/crystal:1.16.3-alpine AS build

WORKDIR /opt/app

COPY shard.yml shard.lock ./
RUN shards

COPY . .
RUN shards build -s --static

FROM scratch

COPY --from=build /opt/app/bin/app /app

CMD ["/app"]
