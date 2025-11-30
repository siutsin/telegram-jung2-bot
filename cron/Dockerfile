FROM node:24 AS build-env
ADD . /app
WORKDIR /app
RUN npm i

FROM gcr.io/distroless/nodejs20-debian12:nonroot
COPY --from=build-env /app /app
WORKDIR /app
CMD ["index.js"]
