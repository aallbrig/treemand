
## Documentation

When creating documentation files:

- Place them in the `docs/` directory or a subdirectory. Typical subdirectories include `docs/design/` and `docs/research/`.
- Prepend a UTC timestamp to the `.md` filename in the format `YYYYMMDD_HHMMSS_UTC_` so files sort chronologically (e.g., `docs/design/20260329_150800_UTC_architecture_overview.md`).


## Agent skills

### Issue tracker

Issues, specs and wayfinder maps live in this repo's GitHub Issues (`gh` CLI). See `docs/agents/issue-tracker.md`.

### Domain docs

Single-context: `GLOSSARY.md` and `docs/adr/` at the repo root. See `docs/agents/domain.md`.

### Vision

`docs/design/20261109_172533_UTC_vision.md` is the guiding document: what treemand is, who it is for, and how competing work is ranked. Read it before choosing or shaping work. It is maintainer-owned; propose changes as `loop:proposed` issues, never edit it.
