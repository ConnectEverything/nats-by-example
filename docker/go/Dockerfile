# Image to build the Go binary.
FROM golang:1.18-alpine AS build

WORKDIR /opt/app

COPY go.mod go.sum ./
RUN go mod download && go mod verify

COPY . ./
RUN go build -v -o /app ./...

# Copy binary to small image for distribution.
FROM alpine

COPY --from=build /app /app

ENTRYPOINT ["/app"]

