FROM node:14 AS build-env
ADD . /app
WORKDIR /app
RUN npm ci --only=production

FROM gcr.io/distroless/nodejs:14
COPY --from=build-env /app /app
WORKDIR /app
ENV DOCKER=true
EXPOSE 3000
CMD ["src/fastify.js"]
