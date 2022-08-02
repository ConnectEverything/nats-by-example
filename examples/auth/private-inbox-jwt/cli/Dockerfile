FROM natsio/nats-box:0.12.0

# Copy nats-server from source image.
COPY --from=nats:2.8.4-alpine /usr/local/bin/nats-server /usr/local/bin/

COPY . .

CMD ["main.sh"]

