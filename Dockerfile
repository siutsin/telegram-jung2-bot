FROM node:6

MAINTAINER Simon Li <li@siutsin.com>

RUN npm install -g yarn
RUN yarn global add pm2

RUN mkdir -p /usr/src/app
WORKDIR /usr/src/app

COPY package.json /usr/src/app/
RUN yarn install

COPY . /usr/src/app

EXPOSE 3001
CMD [ "yarn", "build" ]
CMD [ "pm2-docker", "--json", "process.yml" ]
