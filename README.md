# Full Fluency: Go

Track your Go proficiency from Beginner to Expert, guided and assessed by an AI coding agent.

This repository is meant to be used as a template for your own Go learning path. It contains a skill registry (`skills.yaml`), a protocol for how an agent administers and scores tests (`PROTOCOL.adoc`), and a report generator (`cmd/reportgen`) that renders your current standing into this repo's own `README.md`.

This page you're reading now is the *onboarding* version. The first time you run the report generator, it overwrites this file with your live report. Nothing is lost: everything below only matters once, at setup time, and `PROTOCOL.adoc` remains for the actual rules. In any case, you can always read this document at [its original location](https://github.com/mnohe/full-fluency-go).

## Fork or use as a template?

Both work, but one will leak your data.

- **Use as a template** (GitHub's "Use this template" button) if you want a private, disconnected copy to track your personal progress. This is the default recommendation: every `tests/*/scorecard.yaml` attempt history is personal data (your scores, your mistakes) and a template gives you a clean repo with no upstream link, so you can keep it private.
- **Fork it** at your own risk. A fork keeps a live link back to this repo, and since this repo is public, a GitHub forks will be public too. Anyone can see your scores and attempt history. Nothing wrong with that, if you want to share your progress with the world.

After that, clone it locally and open it in the devcontainer. If you do want to contribute a fix back to the shared tooling (`cmd/reportgen`, `PROTOCOL.adoc`), you can clone this repository and create a pull request.

## Quick start

1. Get your copy as explained above, and open it in the devcontainer.
2. Give an AI coding agent access to the repo and ask it to administer a test, or work through `practice/` on your own. Agents should pick up `AGENTS.md` automatically. You can read `PROTOCOL.adoc` yourself for the actual rules it's following.
3. `go run ./cmd/reportgen` any time to regenerate `README.md` and see where you stand, then commit it along with `skills.yaml` and `tests/*/scorecard.yaml` if you want your progress tracked (and shareable) over time. It is generated, not hand-edited. Always regenerate after logging an attempt rather than editing it directly. The very first run replaces this page with your report.

## How it works

`skills.yaml` is the source of truth, `cmd/reportgen` renders it (plus every `tests/*/scorecard.yaml`) into `README.md` (GitHub-native). `PROTOCOL.adoc` is the actual contract, including grading flow and the skill-state ladder. Read them before changing any of the above.

## License

MIT, see `LICENSE` for details.
