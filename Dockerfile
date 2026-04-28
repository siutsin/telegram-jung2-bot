FROM golang:1.26 AS build-env
WORKDIR /app
COPY go/go.mod go/go.sum ./go/
WORKDIR /app/go
RUN go mod download
COPY go/ ./
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/telegram-jung2-bot .

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build-env /out/telegram-jung2-bot /telegram-jung2-bot
ENV DOCKER=true
EXPOSE 3000
ENTRYPOINT ["/telegram-jung2-bot"]
