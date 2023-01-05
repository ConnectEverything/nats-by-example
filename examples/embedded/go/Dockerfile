FROM golang:1.19-bullseye AS build

WORKDIR /opt/app

COPY go.mod go.sum ./
RUN go mod download && go mod verify

COPY . ./
RUN go build -v -o /app ./...

FROM debian

RUN apt-get -y update && apt-get -y install curl

RUN curl -JLO "https://dl.filippo.io/mkcert/latest?for=linux/amd64"
RUN chmod +x mkcert-v*-linux-amd64
RUN cp mkcert-v*-linux-amd64 /usr/local/bin/mkcert

COPY --from=build /app /app

RUN mkcert \
  -key-file key.pem \
  -cert-file cert.pem \
  localhost

RUN mkcert \
  -client \
  -key-file client-key.pem \
  -cert-file client-cert.pem \
  localhost

RUN ln -s /root/.local/share/mkcert/rootCA.pem ca.pem

CMD ["/app"]

