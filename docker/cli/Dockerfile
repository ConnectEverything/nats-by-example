FROM natsio/nats-box:0.12.0

RUN useradd --create-home --user-group nats
USER nats
WORKDIR /home/nats/code

COPY . .

CMD ["bash", "main.sh"]

