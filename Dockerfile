FROM node:22 AS build-env
ADD . /app
WORKDIR /app
RUN npm i
RUN npm run build

FROM gcr.io/distroless/nodejs20-debian12:nonroot
COPY --from=build-env /app/dist /dist
WORKDIR /dist
ENV DOCKER=true
EXPOSE 3000
CMD ["main.js"]
