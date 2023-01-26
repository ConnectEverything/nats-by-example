FROM golang:1.19-alpine3.17 AS build

RUN apk update && apk add git

RUN go install github.com/nats-io/natscli/nats@v0.0.35
RUN go install github.com/nats-io/nats-server/v2@v2.9.11

FROM alpine:3.17

RUN apk add bash curl postgresql-client jq

COPY --from=build /go/bin/nats-server /usr/local/bin/
COPY --from=build /go/bin/nats /usr/local/bin/

COPY . .

ENTRYPOINT ["bash"]

CMD ["main.sh"]
