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
| `bad-required-outside-default.yml` | 7 | a `${VAR:?}` on a service outside the default profile set, which breaks `docker compose` for everybody (T-397) |
| `bad-env-example-comment.yml` | 7 | an empty variable with an inline comment in the paired `bad-env-example-comment.env`: compose reads the comment as the value, so `${VAR:?}` never fires (T-397) |
| `bad-default-differs-from-manifest.yml` | 8 | a platform variable whose compose default differs from the manifest's — the very `MV_CORE_ADMIN_CLIENTS` line T-411 removed |
| `bad-default-not-a-service.yml` | 8 | an address default that differs from the manifest and does not name a service of the compose network (one item of a list is enough) |
| `bad-default-undeclared.yml` | 8 | a default for a platform variable the manifest does not declare (C-08's old name for the gateway allow-list) |
| `bad-default-not-an-address-variable.yml` | 8 | a client allow-list defaulting to `service:port` of a declared service — the address exception serves only variables whose manifest default is an address (T-411 review #1 Mi-1) |
| `bad-default-address-without-port.yml` | 8 | an address variable defaulting to a bare service name, without a port or a scheme (Mi-1) |
| `bad-nodefault-braced.yml` | 8 | `${MV_X}` with no modifier for a variable whose manifest default is not empty: a silent `.env` hands the process an empty value (Mi-2) |
| `bad-nodefault-bare.yml` | 8 | the same trap spelled `$MV_X` (Mi-2) |
| `bad-secret-passthrough.yml` | 3 | a required credential passed through as a key with no value — the form rule 8 recommends for allow-lists, never right for a secret (T-411) |

A fixture may bring its own example environment: when `bad-x.env` sits next to
`bad-x.yml`, the linter reads it instead of `.env.example` for the `.env`-format
half of rule 7. That keeps `make compose-lint` and the CI step iterating over
one pattern, `bad-*.yml`.

Every value here is a placeholder, never a working credential. The files are
resolved with the same `--env-file` pair as the real compose file, so they must
stay interpolatable with `build/versions.env` and `.github/ci.env` — except
`bad-required-outside-default.yml`, whose whole point is a variable that is
*not* interpolatable on a clean machine (rule 7 runs before the model is built
and rejects the file there).
