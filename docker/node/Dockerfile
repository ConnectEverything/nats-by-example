FROM node:slim

WORKDIR /opt/app

COPY package.json ./

RUN npm install

COPY . .

CMD ["node", "main.js"]
