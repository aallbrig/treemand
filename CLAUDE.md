
## Documentation

When creating documentation files:

- Place them in the `docs/` directory or a subdirectory. Typical subdirectories include `docs/design/` and `docs/research/`.
- Prepend a UTC timestamp to the `.md` filename in the format `YYYYMMDD_HHMMSS_UTC_` so files sort chronologically (e.g., `docs/design/20260329_150800_UTC_architecture_overview.md`).


## Agent skills

### Issue tracker

Issues, specs and wayfinder maps live in this repo's GitHub Issues (`gh` CLI). See `docs/agents/issue-tracker.md`.

### Domain docs

Single-context: `GLOSSARY.md` and `docs/adr/` at the repo root, created lazily. See `docs/agents/domain.md`.
