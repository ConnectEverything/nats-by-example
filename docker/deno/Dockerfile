FROM denoland/deno:1.29.1

COPY . .

RUN deno cache main.ts

CMD ["run", "--allow-env", "--allow-net", "main.ts"]
