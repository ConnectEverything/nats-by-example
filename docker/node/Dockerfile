FROM node:19.1.0-slim

WORKDIR /opt/app

COPY package.json ./

RUN npm install

COPY . .

CMD ["node", "main.js"]
