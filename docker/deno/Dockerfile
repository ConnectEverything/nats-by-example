FROM denoland/deno:2.9.8

COPY . .

RUN deno cache main.js

CMD ["run", "--allow-env", "--allow-net", "main.js"]
