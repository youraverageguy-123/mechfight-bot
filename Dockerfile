# Mech Royale bot (mechfight/1). The match runs it with no network, a read-only root
# (writable /tmp), 1 CPU, 512 MB, uid 65534, and the map mounted at /arena.
FROM oven/bun:1.4.4-slim
WORKDIR /bot
COPY package.json mechfight.ts ./
COPY src ./src
ENTRYPOINT ["bun", "/bot/src/main.ts"]
