FROM denoland/deno:1.28.2

COPY . .

RUN deno cache main.ts

CMD ["run", "--allow-env", "--allow-net", "main.ts"]
