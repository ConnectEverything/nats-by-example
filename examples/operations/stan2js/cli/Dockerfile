FROM golang:1.27-alpine3.23 AS build

RUN apk update && apk add git

RUN go install github.com/nats-io/nats-server/v2@v2.15.3
RUN go install github.com/nats-io/natscli/nats@v0.5.2
RUN go install github.com/nats-io/stan2js/cmd/stan2js@v0.1.0

COPY ./generate-stan-data ./generate-stan-data

RUN cd ./generate-stan-data && go install .

FROM alpine:3.23

RUN apk add bash curl

COPY --from=build /go/bin/nats-server /usr/local/bin/
COPY --from=build /go/bin/nats /usr/local/bin/
COPY --from=build /go/bin/stan2js /usr/local/bin/
COPY --from=build /go/bin/generate-stan-data /usr/local/bin/

COPY . .

ENTRYPOINT ["bash"]

CMD ["main.sh"]
