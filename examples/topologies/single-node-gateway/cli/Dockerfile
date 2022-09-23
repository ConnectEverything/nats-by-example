FROM natsio/nats-box:0.12.0

COPY --from=nats:2.9.0-alpine /usr/local/bin/nats-server /usr/local/bin/

COPY . .

CMD ["main.sh"]

