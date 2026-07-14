# Full Fluency: Go

Track your Go proficiency from Beginner to Expert, guided and assessed by an
AI coding agent.

This repository is meant to be used as a template for your own Go learning
path. It contains a skill registry (`skills.yaml`), a protocol for how an
agent administers and scores tests (`PROTOCOL.adoc`), and a generator
(`cmd/skillsgen`) that renders your current standing into this repo's own
`README.md`. This page you're reading now is the *onboarding* version —
the first time you run `cmd/skillsgen`, it overwrites this file with your
live report. Nothing is lost: everything below only matters once, at setup
time, and `PROTOCOL.adoc` remains for the actual rules.

## Fork or use as a template?

Both work, but one will leak your data.

- **Use as a template** (GitHub's "Use this template" button) if you want a
  private, disconnected copy to track your personal progress. This is the
  default recommendation: `skills.yaml`'s `achieved` flags and every
  `tests/*/scorecard.yaml` attempt history are personal data (your scores,
  your mistakes) and a template gives you a clean repo with no upstream
  link, so you can keep it private.
- **Fork it** at your own risk. A fork keeps a live link back to this repo,
  and if this repo is public, GitHub forks are public too. Anyone can see
  your scores and attempt history. Nothing wrong with that, if you want to share your progress with the world.

Either way, clone it locally and open it in the devcontainer — this isn't
usable from the GitHub web UI alone, and if you do want to contribute a fix
back to the shared tooling (`cmd/skillsgen`, `PROTOCOL.adoc`), that's a
normal pull request from your clone.

## Quick start

1. Get your copy as explained above, and open it in the devcontainer.
2. Give an AI coding agent access to the repo and ask it to administer a
   test, or work through `practice/` on your own. Agents should pick up
   `AGENTS.md` automatically; read `PROTOCOL.adoc` yourself for the actual
   rules it's following.
3. `go run ./cmd/skillsgen` any time to regenerate `README.md`
   and see where you stand, then commit it along with `skills.yaml` and
   `tests/*/scorecard.yaml` if you want your progress tracked (and
   shareable) over time. It is generated, not hand-edited — always
   regenerate after logging an attempt rather than editing it directly.
   The very first run replaces this page with your report.

## How it works

`skills.yaml` is the source of truth, `cmd/skillsgen` renders it (plus every
`tests/*/scorecard.yaml`) into `README.md` (GitHub-native, colored status
via emoji — no asset files to carry into every fork). `PROTOCOL.adoc` is
the actual contract — grading flow, confidence math, everything — read it
before changing any of the above.

## License

MIT — see `LICENSE`.
