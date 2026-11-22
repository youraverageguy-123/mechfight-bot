# Practice build: giddygiddyboy's MIT Go bot (giddy/, see giddy/LICENSE) with our per-slot RNG salts.
FROM golang:1.26.1-bookworm@sha256:ab3d6955bbc813a0f3fdf220c1d817dd89c0b3f283777db8ece4a32fe7858edd AS build
WORKDIR /src
COPY giddy/go.mod ./
COPY giddy/mech ./mech
COPY giddy/cmd/bot ./cmd/bot
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o /mechbot ./cmd/bot

FROM scratch
COPY --from=build /mechbot /mechbot
COPY giddy/LICENSE /LICENSE-giddy
ENTRYPOINT ["/mechbot"]
