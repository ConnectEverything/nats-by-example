# Image to build the Go binary.
FROM golang:1.18-alpine AS build

RUN adduser -D nats
USER nats
WORKDIR /home/nats/code

COPY go.mod go.sum ./
RUN go mod download && go mod verify

COPY . ./
RUN go build -v -o /home/nats/app ./...

# Copy binary to small image for distribution.
FROM alpine

RUN adduser -D nats
USER nats

COPY --from=build /home/nats/app /home/nats/

CMD ["/home/nats/app"]

