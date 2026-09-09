// Archive module: keeps the code out of the root module's ./... pattern (F-3, infrastructure.md 4.6).
// Intentionally without `require`: the archived code is not built, linted or vendored.
module multiverse-core.io/archive

go 1.24
