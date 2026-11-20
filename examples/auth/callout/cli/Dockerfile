FROM golang:1.27-alpine3.23 AS build

RUN apk update && apk add git bash curl

RUN go install github.com/nats-io/nats-server/v2@v2.15.3
RUN go install github.com/nats-io/natscli/nats@v0.5.2
RUN go install github.com/nats-io/nsc/v2@v2.15.0

WORKDIR /usr/src/app

COPY . .

RUN go build -C ./service -v -o /usr/local/bin/service

ENTRYPOINT ["bash"]

CMD ["main.sh"]
