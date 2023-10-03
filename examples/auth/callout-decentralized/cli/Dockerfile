FROM golang:1.21-alpine3.17 AS build

RUN apk update && apk add git bash curl jq

RUN go install github.com/nats-io/nats-server/v2@v2.10.1
RUN go install github.com/nats-io/natscli/nats@v0.1.1
RUN go install github.com/nats-io/nsc/v2@v2.8.1

WORKDIR /usr/src/app

COPY . .

RUN go build -C ./service -v -o /usr/local/bin/service
RUN go build -C ./client -v -o /usr/local/bin/client

ENTRYPOINT ["bash"]

CMD ["main.sh"]
