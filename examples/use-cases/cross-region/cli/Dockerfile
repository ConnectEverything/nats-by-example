FROM natsio/nats-box:0.12.0

RUN apk add bash

COPY --from=synadia/nats-server:nightly /bin/nats-server /usr/local/bin/

COPY . .

ENTRYPOINT ["bash"]

CMD ["main.sh"]
