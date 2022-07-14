FROM node:slim

RUN useradd --create-home --user-group nats
USER nats
WORKDIR /home/nats/code

COPY package.json ./

RUN npm install

COPY . .

CMD ["npm", "run", "main.js"]