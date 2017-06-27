FROM node:latest

MAINTAINER Simon Li <li@siutsin.com>

RUN mkdir -p /usr/src/app
WORKDIR /usr/src/app

COPY package.json /usr/src/app/
RUN npm install

COPY . /usr/src/app

EXPOSE 8443
CMD [ "npm", "start" ]
