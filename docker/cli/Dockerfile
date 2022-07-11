FROM natsio/nats-box:0.12.0

WORKDIR /opt/app

COPY . ./
RUN chmod +x main.sh

ENTRYPOINT ["/opt/app/main.sh"]

