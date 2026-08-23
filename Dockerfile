# Build natively, cross-compile to the target. Avoids running the Go compiler
# under QEMU for the arm64 half of a multi-arch build.
FROM --platform=$BUILDPLATFORM golang:1.27 AS build-env
ARG TARGETARCH
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . ./
RUN CGO_ENABLED=0 GOOS=linux GOARCH=$TARGETARCH go build -o /out/telegram-jung2-bot ./cmd

FROM gcr.io/distroless/static-debian12:nonroot@sha256:afa5c872c891853ca7fcf1f12c3edb23f7eeef36189728842dd51042ff57f7ab
COPY --from=build-env /out/telegram-jung2-bot /telegram-jung2-bot
ENV DOCKER=true
EXPOSE 3000 9090
ENTRYPOINT ["/telegram-jung2-bot"]
