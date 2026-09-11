# testdata/compose-lint

Fixtures of `scripts/compose-lint.sh`. They exist because a linter that silently
stops catching its own class of defect is worse than no linter (review of T-008,
M-4), and a linter that refuses a correct file teaches its operator to ignore it.

- `bad-*.yml` breaks exactly one house rule of `infrastructure.md` §3.1.1. The
  rule is named inside the file, in a line of its header comment:
  `# expect-rule: N`. One or more `# expect-text: ...` lines name the reason — a
  fragment the refusal must contain — so that a fixture is held not only to its
  rule but to the condition within the rule it plants.
- `good-*.yml` is a clean file for a case the linter once got wrong, and must
  pass.

`scripts/compose-lint.sh --fixtures` runs both sets; `make compose-lint` and the
CI job `compose-lint` call it right after the linter has passed on the real
compose files. A `bad-*.yml` fails the run when it passes the linter, when it has
no `# expect-rule:` line, when any other rule fires as well (a fixture red for
two reasons stops proving either the moment one is fixed — T-411 acceptance), or
when an `# expect-text:` fragment is missing from the refusal (T-413). The run
also fails when a fixture carries two `# expect-rule:` lines, when a fixture is
named `*.yaml` (only `*.yml` is read), and when the directory is missing or
holds no bad or no good fixture at all — a moved directory must not turn the
self-test into a silent pass (T-413 review #1 Mi-3, N-2). The last line says how
many of each ran. The directory can be overridden with `COMPOSE_LINT_FIXTURES`,
which is how the self-test itself is tested.

| File | Rule | What it plants |
|---|---|---|
| `bad-latest.yml` | 1 | a third-party image on a floating `latest` tag (NFR-071) |
| `bad-port.yml` | 2 | a published port that is not bound to `127.0.0.1` (SEC-13) |
| `bad-env-list.yml` | 3 | a literal password in an `environment` written as a YAML list (`- KEY=value`) |
| `bad-anchor.yml` | 3 | literal credentials in an anchor other than `x-platform-env` |
| `bad-secret-passthrough.yml` | 3 | a required credential passed through as a key with no value — the form rule 8 recommends for allow-lists, never right for a secret (T-411) |
| `bad-secret-quoted-key.yml` | 3 | the same key with no value, quoted: `"MINIO_ROOT_USER":` (T-411 review #1 N-3) |
| `bad-secret-quoted-list.yml` | 3 | a literal credential in a list item quoted as a whole, `- "KEY=value"` (N-3) |
| `bad-secret-spaced-colon.yml` | 3 | a bare required credential and a literal password with the colons aligned, `KEY : value` — one space before one colon, several before the other (T-413 review #1 Ma-1, review #2 N-1) |
| `bad-secret-escaped.yml` | 3 | `$${MINIO_ROOT_PASSWORD:?...}` — compose's escape, so the container gets the literal text as its password (T-413) |
| `bad-required-outside-default.yml` | 7 | a `${VAR:?}` on a service outside the default profile set, which breaks `docker compose` for everybody (T-397) |
| `bad-env-example-comment.yml` | 7 | an empty variable with an inline comment in the paired `bad-env-example-comment.env`: compose reads the comment as the value, so `${VAR:?}` never fires (T-397) |
| `bad-required-unmarked.yml` | 7 | a `${VAR:?}` of the always-loaded file whose variable the paired `bad-required-unmarked.env` does not mark `[required]` — the operator following the README skips it (T-412 acceptance, T-413) |
| `bad-required-nested.yml` | 7 | a `:?` nested in a default, `${FOO:-${MV_X:?}}`, whose variable the paired `bad-required-nested.env` does not mark (review #1 Mi-2) |
| `bad-default-differs-from-manifest.yml` | 8 | a platform variable whose compose default differs from the manifest's — the very `MV_CORE_ADMIN_CLIENTS` line T-411 removed |
| `bad-default-undeclared.yml` | 8 | a default for a platform variable the manifest does not declare (C-08's old name for the gateway allow-list) |
| `bad-default-not-an-address-variable.yml` | 8 | a client allow-list defaulting to `service:port` of a declared service — only the explicit set of network addresses may name a service (T-411 review #1 Mi-1, T-413) |
| `bad-default-not-a-service.yml` | 8 | a network address that does not name a service of the compose network (one item of a list is enough) |
| `bad-default-address-without-port.yml` | 8 | a network address that is a bare service name, not in the shape of the manifest's default (Mi-1) |
| `bad-network-shape.yml` | 8 | `MV_MINIO_ENDPOINT` with a scheme, where the manifest's default is host:port (T-411 review #2 N-4) |
| `bad-network-core-addr.yml` | 8 | `MV_CORE_ADDR`, a listen address, defaulting to a service of the network (N-4) |
| `bad-network-outside-set.yml` | 8 | a network variable outside the explicit set; the refusal says to add it there (N-4) |
| `bad-ollama-default.yml` | 8 | an `OLLAMA_*` default different from the manifest's `DeclareExternal` (contracts.md §16 p. 5) |
| `bad-ollama-passthrough.yml` | 8 | an `OLLAMA_*` key with no value — the container would run on the image's default, not ours |
| `bad-ollama-spaced-colon.yml` | 8 | the same with the colons aligned: `OLLAMA_KEEP_ALIVE :` with one space, `OLLAMA_NUM_PARALLEL      :` with several (Ma-1, review #2 N-1) |
| `bad-ollama-nodefault.yml` | 8 | `${OLLAMA_X}` with no modifier: the container gets an empty value instead of the manifest's (Mi-1) |
| `bad-comment-in-quotes.yml` | 8 | `${MV_X}` after a `#` inside quotes — text, not a comment, and compose interpolates it (Mi-2) |
| `bad-comment-in-single-quotes.yml` | 8 | the same inside single quotes; a separate file, since the double-quoted one gives the same refusal (review #2 N-2) |
| `bad-nodefault-braced.yml` | 8 | `${MV_X}` with no modifier for a variable whose manifest default is not empty: a silent `.env` hands the process an empty value (Mi-2) |
| `bad-nodefault-bare.yml` | 8 | the same trap spelled `$MV_X` (Mi-2) |
| `bad-nodefault-after-escape.yml` | 8 | the same trap after compose's escape: `$$$MV_X` (N-6) |
| `bad-nodefault-in-command.yml` | 8 | the same trap inside a `command:`, where the advice is not "a key with no value" (N-5) |
| `bad-alternate-colon.yml` | 8 | `${MV_X:+x}`: compose's own text when set, empty when `.env` is silent (N-6) |
| `bad-alternate-plain.yml` | 8 | `${MV_X+x}`, the same without the colon (N-6) |
| `bad-default-nested-outer.yml` | 8 | a default that is another interpolation, `${MV_X:-${Y:-d}}` (T-411 review #1 N-1) |
| `bad-default-nested-inner.yml` | 8 | a wrong platform default nested inside an as-is variable's default (N-1) |
| `good-network-set.yml` | — | the six network addresses naming services in the right shape, and `MV_LLM_URL`, `MV_TELEGRAM_HEALTH_ADDR`, `MV_MEMORY_URL` that the old guess mistook (N-4) |
| `good-escaped-dollar.yml` | — | `$${MV_X:-d}` and `$$MV_X` in a command: text for the container's shell (N-1) |
| `good-comment-after-bare-key.yml` | — | keys with no value followed by a YAML comment, one quoting `${MV_X}` (N-5) |
| `good-quoted-keys.yml` | — | required credentials under quoted keys and quoted list items (N-3) |
| `good-ollama-defaults.yml` | — | the `OLLAMA_*` block with the manifest's defaults |
| `good-spaced-colon.yml` | — | the correct forms with the colons aligned (Ma-1) |
| `good-compose-project-name.yml` | — | `${COMPOSE_PROJECT_NAME}` with no modifier: compose sets it itself, and the contract holds compose to third-party defaults for `OLLAMA_*` only (Mi-1) |

A fixture may bring its own example environment: when `x.env` sits next to
`x.yml`, the linter reads it instead of `.env.example` for rule 7 — both the
clean machine and the `[required]` markers. That keeps the fixtures iterating
over one pattern per kind.

Every value here is a placeholder, never a working credential. The files are
resolved with the same `--env-file` pair as the real compose file, so they must
stay interpolatable with `build/versions.env` and `.github/ci.env` — except
`bad-required-outside-default.yml`, whose whole point is a variable that is
*not* interpolatable on a clean machine (rule 7 runs before the model is built
and rejects the file there).
