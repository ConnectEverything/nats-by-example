FROM golang:1.27-alpine3.23 AS build

WORKDIR /opt/app

COPY go.mod go.sum ./
RUN go mod download && go mod verify

COPY . ./
RUN go build -v -o /app ./...

FROM alpine:3.23

COPY --from=build /app /app

CMD ["/app"]

