# Build natively, cross-compile to the target. Avoids running the Go compiler
# under QEMU for the arm64 half of a multi-arch build.
FROM --platform=$BUILDPLATFORM golang:1.27@sha256:205c7636d74e0ffbe3d007162df7d74f859f5a8c8322dfb9f907977686c126cf AS build-env
ARG TARGETARCH
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY cmd ./cmd
COPY internal ./internal
RUN CGO_ENABLED=0 GOOS=linux GOARCH=$TARGETARCH go build -o /out/telegram-jung2-bot ./cmd

FROM gcr.io/distroless/static-debian12:nonroot@sha256:afa5c872c891853ca7fcf1f12c3edb23f7eeef36189728842dd51042ff57f7ab
COPY --from=build-env /out/telegram-jung2-bot /telegram-jung2-bot
ENV DOCKER=true
EXPOSE 3000 9090
ENTRYPOINT ["/telegram-jung2-bot"]
