FROM denoland/deno:1.23.3

WORKDIR /opt/app

COPY . .

RUN deno cache main.ts

CMD ["run", "--allow-env", "--allow-net", "main.ts"]
