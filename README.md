# ohio-ironclad (Mech Royale practice build)

Practice entry for the Builders Buildathon Mech Royale track (`mechfight/1`).

## Credit and disclosure

The bot in `cmd/`, `internal/`, `go.mod` and the `Dockerfile` is **Apex-Go v3 by ahmad-ovh (team Ahmad
Suliman)**, used under its MIT License (kept in `LICENSE-apex`, copyright (c) 2026 ahmad-ovh).
Source: https://github.com/ahmad-ovh/Buildathon, commit `56e0f76`. Apex-Go itself credits the public bots
it builds on (andrew-zhouu, lukejwaszyn, MarwanMoh11, SandeepMadhavarapu and others) in its code comments.
We did not write this bot. On top of it we changed the display name and some practice settings (listed in
each commit message), and wrote three practice-squad additions of our own, each behind a setting in
`internal/bot/bot.go` (marked "ohio-ironclad"; code in `internal/bot/policy.go`):

- kill pacing (`PaceSecs`, `PaceHP`): every kill brings another house hunter, so after a kill open no new
  hunt or seek until health has regenerated;
- full-magazine hunts (`HuntAmmo`): top up the gun while watching an idle mech, and open hunts with a
  full magazine;
- wounded-first seeking (`SeekHurtW`): when seeking, prefer a damaged mech over a slightly nearer one.
