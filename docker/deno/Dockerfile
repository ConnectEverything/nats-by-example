FROM denoland/deno:1.23.3

RUN useradd --create-home --user-group nats
USER nats
WORKDIR /home/nats/code

COPY . .

RUN deno cache main.ts

CMD ["run", "--allow-env", "--allow-net", "main.ts"]
