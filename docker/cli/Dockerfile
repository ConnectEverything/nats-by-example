FROM golang:1.27-alpine3.23 AS build

RUN apk update && apk add git

RUN go install github.com/nats-io/nats-server/v2@v2.10.1
RUN go install github.com/nats-io/natscli/nats@v0.1.1
RUN go install github.com/nats-io/nsc/v2@v2.8.1

FROM alpine:3.23

RUN apk add bash curl

COPY --from=build /go/bin/nats-server /usr/local/bin/
COPY --from=build /go/bin/nats /usr/local/bin/
COPY --from=build /go/bin/nsc /usr/local/bin/

COPY . .

ENTRYPOINT ["bash"]

CMD ["main.sh"]
