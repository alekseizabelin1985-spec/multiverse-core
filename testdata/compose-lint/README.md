# testdata/compose-lint

Negative fixtures for `scripts/compose-lint.sh`. Each file breaks exactly one
house rule of `infrastructure.md` §3.1.1, and `make compose-lint` asserts that
the linter exits non-zero on every one of them right after it has passed on the
real `docker-compose.yml`. They exist because a linter that silently stops
catching its own class of defect is worse than no linter (review of T-008, M-4).

| File | Rule | What it plants |
|---|---|---|
| `bad-env-list.yml` | 3 | a literal password in an `environment` written as a YAML list (`- KEY=value`) |
| `bad-anchor.yml` | 3 | literal credentials in an anchor other than `x-platform-env` |
| `bad-port.yml` | 2 | a published port that is not bound to `127.0.0.1` (SEC-13) |
| `bad-latest.yml` | 1 | a third-party image on a floating `latest` tag (NFR-071) |

Every value here is a placeholder, never a working credential. The files are
resolved with the same `--env-file` pair as the real compose file, so they must
stay interpolatable with `build/versions.env` and `.github/ci.env`.
