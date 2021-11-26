FROM node:16 AS build-env
ADD . /app
WORKDIR /app
RUN npm i
RUN npm run build

FROM gcr.io/distroless/nodejs:16
COPY --from=build-env /app/dist /dist
WORKDIR /dist
ENV DOCKER=true
EXPOSE 3000
CMD ["main.js"]
