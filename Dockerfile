# Mech Royale bot: static Go binary, no runtime dependencies.
# Build: docker build -t mech-bot .
# The match runs it with no network, read-only root, 1 CPU, 512 MB, uid 65534, map at /arena.
# Base image pinned by digest so a later rebuild gets exactly the same toolchain.
FROM golang:1.25.4-alpine3.22@sha256:d3f0cf7723f3429e3f9ed846243970b20a2de7bae6a5b66fc5914e228d831bbb AS build
WORKDIR /src
COPY go.mod ./
COPY cmd/bot ./cmd/bot
COPY internal/bot ./internal/bot
COPY internal/proto ./internal/proto
COPY internal/world ./internal/world
ENV CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GOFLAGS=-mod=mod GOTOOLCHAIN=local
RUN go build -trimpath -ldflags="-s -w" -o /out/bot ./cmd/bot

FROM scratch
COPY --from=build /out/bot /bot
USER 65534:65534
ENTRYPOINT ["/bot"]
