FROM natsio/nats-box:0.12.0

RUN adduser -D nats
USER nats
RUN mkdir /home/nats/code

WORKDIR /home/nats/code

COPY . .

CMD ["main.sh"]

