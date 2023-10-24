FROM golang:1.21-alpine3.18 AS build

RUN apk update && apk add git

RUN go install github.com/nats-io/nats-server/v2@v2.10.3
RUN go install github.com/nats-io/natscli/nats@v0.0.35
RUN go install github.com/nats-io/stan2js/cmd/stan2js@v0.1.0

COPY ./generate-stan-data ./generate-stan-data

RUN cd ./generate-stan-data && go install .

FROM alpine:3.18

RUN apk add bash curl

COPY --from=build /go/bin/nats-server /usr/local/bin/
COPY --from=build /go/bin/nats /usr/local/bin/
COPY --from=build /go/bin/stan2js /usr/local/bin/
COPY --from=build /go/bin/generate-stan-data /usr/local/bin/

COPY . .

ENTRYPOINT ["bash"]

CMD ["main.sh"]
