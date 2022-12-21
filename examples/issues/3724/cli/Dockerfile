FROM natsio/nats-box:0.13.3

RUN apk add bash curl

COPY --from=nats:2.9.7 /nats-server /usr/local/bin/

COPY . .

ENTRYPOINT ["bash"]

CMD ["main.sh"]
