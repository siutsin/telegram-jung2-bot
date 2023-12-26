FROM node:20 AS build-env
ADD . /app
WORKDIR /app
RUN npm i

FROM gcr.io/distroless/nodejs:20
COPY --from=build-env /app /app
WORKDIR /app
CMD ["index.js"]
