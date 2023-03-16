FROM golang:1.19.4-alpine3.17 AS build

WORKDIR /opt/app

COPY go.mod go.sum ./
RUN go mod download && go mod verify

COPY . ./
RUN go build -v -o /app ./...

FROM alpine

COPY --from=build /app /app

CMD ["/app"]

