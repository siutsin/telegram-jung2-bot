FROM node:20 AS build-env
ADD . /app
WORKDIR /app
RUN npm i
RUN npm run build

FROM gcr.io/distroless/nodejs:20
COPY --from=build-env /app/dist /dist
WORKDIR /dist
ENV DOCKER=true
EXPOSE 3000
CMD ["main.js"]
