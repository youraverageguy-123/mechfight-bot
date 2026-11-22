# ohio-ironclad (practice build)

This branch is a credited copy of the MIT-licensed Mech Royale bot at
https://github.com/andrew-zhouu/builders-mech-royale-2026 (commit `6f9e67868d53a848747e2eebc6746fcc33a1ef13`,
v0.167). The license and copyright notice are kept unchanged in `LICENSE`. Our changes are in this
branch's commit history (the display name, and any parameter changes noted there).

---

# Mech Royale bot

Our entry for the Builders Buildathon **Mech Royale** track: a `mechfight/1` bot that pilots one mech
in the 16-mech battle royale. It reads observations as JSON lines on stdin and answers each with one
action line on stdout.

Written in Go using only the standard library. There is no third-party code and no code from before
the event in this repository.

## Layout

| Path | What it is |
| --- | --- |
| `Dockerfile` | Two-stage build: compiles a static binary, ships it in an empty image |
| `cmd/bot` | The stdin/stdout protocol loop (answers the newest observation, echoes its tick, never prints anything else to stdout) |
| `internal/proto` | `mechfight/1` message types and the action encoder |
| `internal/world` | Map loading (`/arena/v2`), ray casts and line of sight, the walk grid and path search, rooftop regions |
| `internal/bot` | The agent: memory of what it has seen, aiming and ballistics, movement and dodging, zone handling, strategy |
| `VERSION` | Label and source revision of this snapshot |

## Build and check

The build needs only Docker. The Go toolchain image is pinned by digest in the `Dockerfile`, and
there are no module dependencies to download, so a rebuild produces the same bot.

```sh
docker build --platform linux/amd64 -t mech-bot .
```

Run the official conformance check from the bot kit (`mechfight-kit`) against the image, with the
same sandbox flags a match uses:

```sh
# from the kit folder (it contains check/ and arena/)
python3 check/check.py --map-dir /arena -- docker run --rm -i --network none --cpus 1 --memory 512m \
  --read-only --tmpfs /tmp:rw,size=64m --user 65534:65534 -v "$PWD/arena:/arena:ro" mech-bot
```

Without Docker, with Go 1.25 installed:

```sh
go build -o bot ./cmd/bot
python3 check/check.py -- ./bot        # run from the kit folder, with the path to ./bot
```

## Runtime behaviour

- Starts in about a second (it precomputes its maps of the arena before answering `hello`).
- Typical decision time is well under a millisecond; path searches are capped so a decision stays far
  inside the 50 ms budget on one CPU.
- Uses no network and writes no files. Memory use stays far below the 512 MB limit.
- If the map cannot be loaded it falls back to a simple bot instead of exiting, and a failure inside a
  decision is caught and answered with the previous movement, so the mech never goes silent.

## License

Released under the MIT License (see `LICENSE`).
