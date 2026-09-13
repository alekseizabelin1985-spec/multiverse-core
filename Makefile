# =============================================================================
# Makefile — the operator's entry point (infrastructure.md v0.3 §2.2)
# =============================================================================
# One target, one action. Rules of this file:
#   * `docker compose`, never `docker-compose` (v1);
#   * no image tag is ever written into a recipe — every version comes from
#     build/versions.env, which is included below (NFR-071);
#   * GNU make 4.3+ with bash (CI: ubuntu-latest, 4.3 — the job `race` runs
#     `make test-race`, T-401), so nothing newer than 4.3 goes into this file;
#     `winget install ezwinports.make` on Windows, or `wsl make <target>`
#     without installing anything (§2.1);
#   * the LLM is a native process outside compose: `make up` never starts it and
#     `make down` never stops it — `make llm-up` / `make llm-down` do
#     (ADR-005 add. 2 p. 7).
# =============================================================================

SHELL := bash
.SHELLFLAGS := -euo pipefail -c
.ONESHELL:
.DEFAULT_GOAL := help

PROJECT_NAME := multiverse-core

# The single source of pinned versions: GO_VERSION, *_IMAGE, MINIO_*,
# LLAMACPP_BUILD, LLM_MODEL_DEFAULT, ALPINE_IMAGE, tool versions.
include build/versions.env

GIT_SHA := $(shell git rev-parse --short HEAD)

# `go build` fails on a path with Cyrillic characters unless the VCS stamp is
# switched off (audit-facts §1); the version is injected with -ldflags instead,
# so nothing is lost. CI does not set this.
export GOFLAGS := -buildvcs=false
LDFLAGS := -X main.version=$(GIT_SHA) -X multiverse-core.io/shared/logging.Version=$(GIT_SHA)

# Compose reads .env and build/versions.env only because of this export.
# Compose takes COMPOSE_ENV_FILES from the environment of its own process, never
# from .env: the variable names the env files, so it cannot come from one of
# them. The line in .env is therefore inert, and a bare `docker compose` without
# it stops on the first *_IMAGE variable (T-412, checked on compose v5.2) —
# which is why the stack is run through make. A value already in the
# environment wins (§4.2).
COMPOSE_ENV_FILES ?= .env,build/versions.env
export COMPOSE_ENV_FILES

# `make up PROFILES=memory,bot` overrides COMPOSE_PROFILES from .env for one
# invocation (§1.3).
COMMA := ,
PROFILES ?=
PROFILE_ARGS := $(foreach p,$(subst $(COMMA), ,$(PROFILES)),--profile $(p))

# Two profiles live in a compose file of their own (T-397): `docker compose`
# interpolates a file whole, before it filters by profile, so a required
# variable of a service outside the default set — the bot token, the unpinned
# Chroma image (D-3) — used to break `make up` for everyone. The files are added
# here, and only when their profile is actually asked for, so that the loud
# refusal stays with the operator who wants that profile.
#
# The active set is `PROFILES=` when it is given, otherwise COMPOSE_PROFILES —
# from the environment, or from .env, which is where §1.3 tells the operator to
# put it. The value in .env may carry a trailing `# comment`, exactly as
# .env.example ships it, and compose's own dotenv parser strips it; so does this.
#
# The three details below are not style: each one made `make` see fewer files
# than compose sees, and the stack then came up silently without the service of
# the profile compose thought was active (T-397 review, Mi-1).
#   tail -n 1  — a duplicated key: compose keeps the last assignment, not the first.
#   export …   — `export COMPOSE_PROFILES=…` is a valid dotenv line; compose reads it.
#   leading ws — compose ignores indentation before the key; a plain ^ anchor does not.
DOTENV_PROFILES := $(shell sed -n 's/^[[:space:]]*\(export[[:space:]][[:space:]]*\)\?COMPOSE_PROFILES=//p' .env 2>/dev/null | tail -n 1 | sed -e 's/[[:space:]][[:space:]]*\#.*$$//' -e 's/^[[:space:]]*//' -e 's/[[:space:]]*$$//' -e 's/^"\(.*\)"$$/\1/' -e "s/^'\(.*\)'$$/\1/")
ACTIVE_PROFILES := $(if $(PROFILES),$(PROFILES),$(if $(COMPOSE_PROFILES),$(COMPOSE_PROFILES),$(DOTENV_PROFILES)))
ACTIVE_PROFILE_LIST := $(subst $(COMMA), ,$(ACTIVE_PROFILES))

COMPOSE_FILES := -f docker-compose.yml
COMPOSE_FILES += $(if $(filter bot,$(ACTIVE_PROFILE_LIST)),-f docker-compose.bot.yml)
COMPOSE_FILES += $(if $(filter legacy,$(ACTIVE_PROFILE_LIST)),-f docker-compose.legacy.yml)
# An explicit `-f` switches off compose's own file discovery, and with it the
# local, git-ignored override the header of docker-compose.yml points the
# operator at (ОВ-40). It stays last, which is what makes it an override.
COMPOSE_FILES += $(if $(wildcard docker-compose.override.yml),-f docker-compose.override.yml)

COMPOSE := docker compose $(COMPOSE_FILES) $(PROFILE_ARGS)

# gitleaks scans the range of the branch until the history is rewritten (§3.1).
# The range starts at the gitflow trunk (T-453). integration/mvp-1 stayed at
# F-0, so its range held every commit since and grew with each task; a task or
# an epic branch is measured against develop, and develop itself against
# BASE=main. CI never calls this target: its security job scans on its own.
BASE ?= develop

# Backups live outside the repository and outside the Docker volumes (§5.6).
BACKUP_DIR ?= $(HOME)/multiverse-backups

# llama-server runs natively on Windows (GPU, D:\Models\...), so pwsh 7 is the
# primary path and the .sh twin is for a Linux/WSL stand (§2.1, §6.2). Falling
# back to the .sh on Windows would be worse than failing: it would try to
# supervise a Windows process with MSYS signals.
UNAME_S := $(shell uname -s 2>/dev/null)
ON_WINDOWS := $(findstring MINGW,$(UNAME_S))$(findstring MSYS,$(UNAME_S))$(findstring CYGWIN,$(UNAME_S))
ROUTER ?=

# `make health` is strict by §2.2 (non-zero unless everything is ok), but a
# missing LLM is a documented degradation, not a broken stack (FR-080/NFR-072),
# so `up`, `deploy` and `rollback` call it with LLM_STRICT=0 (§6.3).
LLM_STRICT ?= 1

# Compose reads .env by itself; make does not. The targets that hand MV_LLM_* to
# a native process therefore load the file the same way `make run-local` does
# (§4.3, §6.2). Values with backslashes (D:\Models\...) must be quoted in .env —
# see the comment above the block in .env.example.
define load_dotenv
if [ ! -f .env ]; then
	echo "$@: .env is missing — copy .env.example to .env and fill the MV_LLM_* block (infrastructure.md §4.2)" >&2
	exit 1
fi
set -a
. ./.env
set +a
endef

# $(call llm,<up|down|health>)
define llm
$(load_dotenv)
if command -v pwsh >/dev/null 2>&1; then
	pwsh scripts/llm-server.ps1 -Action $(1) $(if $(ROUTER),-Router)
elif [ -n "$(ON_WINDOWS)" ]; then
	echo "llm: PowerShell 7 is required here — llama-server is a native Windows process with CUDA and D:\Models paths (infrastructure.md §2.1, §6.2)." >&2
	echo "     winget install Microsoft.PowerShell   (or run the target under wsl)" >&2
	exit 1
else
	bash scripts/llm-server.sh $(1) $(if $(ROUTER),--router)
fi
endef

# -----------------------------------------------------------------------------
# Build and test
# -----------------------------------------------------------------------------
.PHONY: build
build: ## Build every binary of the platform into bin/
	@go build -ldflags "$(LDFLAGS)" -o bin/ ./cmd/...

.PHONY: lint
lint: ## golangci-lint over the module
	@golangci-lint run

# ОВ-5 (ADR-012): the race detector runs in CI on Linux, and locally the target
# runs without it. It is not a preference — -race needs cgo and a C toolchain,
# and on a machine without one the flag does not weaken the run, it aborts the
# whole target before a single test executes ("-race requires cgo"). Silently
# dropping it would be worse than the flag: the line below says out loud which
# of the two runs happened, so nobody reads a green local run as a race-checked
# one.
RACE := $(shell go env CGO_ENABLED 2>/dev/null | grep -q '^1$$' && command -v "$$(go env CC 2>/dev/null)" >/dev/null 2>&1 && echo -race)

.PHONY: test
test: ## Unit tests: short, no network; -race only where cgo is available (ОВ-5)
	@$(if $(RACE),echo "test: with the race detector",echo "test: WITHOUT the race detector (no cgo toolchain); CI runs it on Linux — ОВ-5")
	@go test -short $(RACE) -count=1 -coverprofile=coverage.out ./...
	# The floor CI applies (job `unit`); packages that do not exist yet are a
	# warning, not a failure (ADR-010 p. 5). Through bash and not as a program:
	# the executable bit lives in the tree, and a script that loses it fails
	# with Permission denied on Linux while Git for Windows shows nothing.
	bash scripts/coverage-gate.sh 60 internal/state internal/mechanics internal/swarm internal/llm internal/replay

# The same flag and the same timeout as the CI job `integration` (T-401): the
# contract set on the live broker, its Close case included, is reachable by the
# race detector only here. 20m and not 15m for the reason the job gives — a
# fully red set walks through every one of its 15-second waits.
.PHONY: test-integration
test-integration: minio-image ## testcontainers: Redpanda, MinIO, Qdrant, Neo4j; -race only where cgo is available
	@$(if $(RACE),echo "test-integration: with the race detector",echo "test-integration: WITHOUT the race detector (no cgo toolchain); the CI job integration runs it on Linux")
	go test -tags integration $(RACE) -count=1 -timeout 20m ./...

.PHONY: test-e2e
test-e2e: ## End to end in one process, in-memory bus
	@go test -tags e2e -count=1 -timeout 10m ./...

# The CI job `race` (T-401) runs exactly this target, and so does an operator
# with cgo: RACE_PKGS and RACE_COUNT below are the only copy of the package
# list and of the repeat count (review #1 of T-401, Mi-1). The detector reports
# only the races that happen during a run, so the packages where goroutines
# meet run RACE_COUNT times; the e2e set runs once, and GOFLAGS carries -race
# into the `go build` of cmd/multiverse it starts as a child.
#
# -timeout bounds one test binary, and with -count every pass of a package runs
# inside the same binary: the 10m is for all the passes of one package
# together. The slowest of them (testkit/gateway) takes seconds per pass
# without the detector.
#
# Without cgo the target refuses: someone who asked for the detector must not
# read a green run as a checked one, and on the runner that refusal is the loud
# failure CI wants. Inside `make ci` it steps aside instead
# (`ci: RACE_OPTIONAL := 1` below), so that ci stays usable on Windows; the
# message says which of the two happened. RACE_OPTIONAL is assigned here, so a
# value exported in the shell cannot soften a direct call; the command line
# still overrides it. English only: the Windows console renders the UTF-8 of
# this file as CP1251 (T-403).
RACE_PKGS := ./shared/eventbus/... ./shared/testkit/... ./shared/runtime/... ./shared/clock/...
RACE_COUNT ?= 3
RACE_OPTIONAL :=

.PHONY: test-race
test-race: ## Race detector, repeated, over the concurrent packages and e2e (CI job race); needs cgo
	@if [ -z "$(RACE)" ]; then
		# Name only the half of the condition that failed. Without a compiler
		# Go itself defaults CGO_ENABLED to 0, so the compiler is checked first.
		cc=$$(go env CC)
		if ! command -v "$$cc" >/dev/null 2>&1; then
			reason="the C compiler CC=$$cc is not on PATH (without one Go defaults CGO_ENABLED to 0)"
		else
			reason="CGO_ENABLED=$$(go env CGO_ENABLED) although $$cc is on PATH (see the environment and go env -w)"
		fi
		if [ -n "$(RACE_OPTIONAL)" ]; then
			echo "test-race: SKIPPED - $$reason; the CI job race runs it on Linux (decision OV-5)" >&2
			exit 0
		fi
		echo "test-race: -race needs cgo and a C compiler: $$reason." >&2
		echo "           The CI job race runs it on Linux; locally use WSL or a gcc on PATH (decision OV-5)." >&2
		exit 1
	fi
	# The two commands are printed so that the log of the CI job shows the flag
	# and the package list it actually ran with.
	echo "test-race: go test -race -count=$(RACE_COUNT) -timeout 10m $(RACE_PKGS)"
	go test -race -count=$(RACE_COUNT) -timeout 10m $(RACE_PKGS)
	# One value for the log line and the run: the file exports
	# GOFLAGS=-buildvcs=false, and the log must show what the child build of
	# e2e really gets, not a shorthand of it (review #2 of T-401, N-7).
	e2e_goflags="$$GOFLAGS -race"
	echo "test-race: GOFLAGS='$$e2e_goflags' go test -race -tags e2e -count=1 -timeout 10m ./test/e2e/..."
	GOFLAGS="$$e2e_goflags" go test -race -tags e2e -count=1 -timeout 10m ./test/e2e/...

# `mvctl blueprint validate blueprints/` joins this target together with the
# command itself, in EPIC-003: the name is reserved in cmd/mvctl/main.go and
# exits with the usage code until then, which would fail the target for a
# command that was never written.
.PHONY: contracts
contracts: ## Schemas and the env manifest agree with the code
	@go run ./cmd/mvctl contracts check
	go run ./cmd/mvctl env check
	go test ./shared/contracts/... -run TestSchemasValid

.PHONY: secrets-scan
secrets-scan: ## gitleaks over the branch range and the content of the index
	@if ! git rev-parse --verify --quiet "$(BASE)^{commit}" >/dev/null; then
		echo "secrets-scan: BASE=$(BASE) is not a commit of this repository; pass BASE=<trunk of the branch>" >&2
		exit 1
	fi
	# gitleaks reports "0 commits scanned" and exits 0 for a range it cannot
	# resolve, so a BASE missing from a clone used to pass the history half
	# without looking at a single commit (T-453). An empty range is only a
	# warning: on develop itself, or on a branch already merged into BASE.
	if [ "$$(git rev-list --count "$(BASE)..HEAD")" -eq 0 ]; then
		echo "secrets-scan: warning: $(BASE)..HEAD holds no commits, the history half scans nothing; on develop itself pass BASE=main" >&2
	fi
	gitleaks git --no-banner --redact --log-opts="$(BASE)..HEAD" .
	# The second scan reads the content of the index, not the work tree: the
	# untracked .env of the owner, build/.legacy-src/ and the worktrees of the
	# agents are not what CI checks out, and they keep the target red for files
	# that are never committed (decision ОВ-8). The paths stay relative so that
	# the fingerprints of .gitleaksignore match (§4.5 p. 1).
	staged=$$(mktemp -d)
	trap 'rm -rf "$$staged"' EXIT
	git checkout-index -a --prefix="$$staged/"
	cd "$$staged" && gitleaks dir --no-banner --redact --config=.gitleaks.toml .

.PHONY: privacy-scan
privacy-scan: ## External identifiers in the artefacts that are committed (NFR-041)
	@go run ./cmd/mvctl privacy scan testdata/

.PHONY: vuln
vuln: ## govulncheck over the module
	@govulncheck ./...

.PHONY: compose-lint
compose-lint: ## The eight house rules of the compose files (§3.1.1), then the linter's own fixtures
	@scripts/compose-lint.sh
	# A rule that quietly stopped firing looks exactly like a clean file, and a
	# fixture rejected by another rule proves nothing about its own: every
	# bad-*.yml must be rejected by exactly the rule of its `# expect-rule:`
	# line, for the reason of its `# expect-text:` lines, and every good-*.yml
	# must pass (testdata/compose-lint, T-413). The same call runs in CI.
	scripts/compose-lint.sh --fixtures

# The parity stand of the two implementations of the LLM scripts (T-405):
# scripts/llm-server.{sh,ps1} and scripts/llm-bench.{sh,ps1} with their modules
# in scripts/lib get the same inputs, and their exit codes, messages, the argv
# they hand to llama-server, the requests they send, the CSV and the fields of
# the JSON report must agree. A double stands in for llama-server; no Docker, no
# network beyond loopback, no real server, and nothing on 8888 is touched. Without
# pwsh on PATH the stand runs the bash half against the expectations of every
# scenario and compares the message texts of the two files statically — its
# first lines say which of the two runs happened. The stand lives in
# testdata/script-parity (outside every ./... pattern, see its package comment);
# PARITY_ARGS passes flags through, e.g. PARITY_ARGS='-run U0 -v'.
PARITY_ARGS ?=

.PHONY: scripts-parity
scripts-parity: ## Same input, same output from the .sh and .ps1 LLM scripts (both halves with pwsh, the bash half without)
	@# The linter does not see testdata/; vet at least keeps the stand honest (T-405 review #1, N-1).
	go vet ./testdata/script-parity
	go run ./testdata/script-parity $(PARITY_ARGS)

# Every control mutant must turn the stand red: the scripts are broken one
# defect at a time in a scratch copy, the control (a syntax error) first and the
# identity mutant (no change, must stay green) second. Not part of ci: it is the
# check of the stand itself, run when the stand or its scenarios change.
.PHONY: parity-mutants
parity-mutants: ## The control mutants of scripts-parity: each must turn the stand red
	@go run ./testdata/script-parity -mutants $(PARITY_ARGS)

.PHONY: ci
ci: lint test test-race contracts secrets-scan privacy-scan vuln compose-lint scripts-parity test-e2e ## Everything CI runs without Docker

# `make ci` is the owner's check on Windows, where -race cannot run: there
# test-race prints SKIPPED and steps aside instead of failing the whole run
# (T-401). A target-specific value reaches the prerequisites of ci, and deploy
# through ci; `make ci RACE_OPTIONAL=` turns the skip back into a refusal.
ci: RACE_OPTIONAL := 1

.PHONY: ci-full
ci-full: ci test-integration ## `make ci` plus the container tests

# -----------------------------------------------------------------------------
# Images
# -----------------------------------------------------------------------------
.PHONY: image
image: ## Platform image: one image, every binary
	@docker build -f build/Dockerfile \
		--build-arg GO_VERSION=$(GO_VERSION) --build-arg VERSION=$(GIT_SHA) \
		-t multiverse-core:$(GIT_SHA) -t multiverse-core:dev .

.PHONY: minio-image
minio-image: ## MinIO built from source (ADR-021 variant B); ~5 min the first time
	@docker build -f build/minio.Dockerfile \
		--build-arg MINIO_TAG=$(MINIO_TAG) \
		--build-arg MINIO_REPO=$(MINIO_REPO) \
		--build-arg MINIO_BUILDER_IMAGE=$(MINIO_BUILDER_IMAGE) \
		-t $(MINIO_IMAGE) .

# -----------------------------------------------------------------------------
# Stack
# -----------------------------------------------------------------------------
.PHONY: up
up: $(if $(filter legacy,$(ACTIVE_PROFILE_LIST)),legacy-src) ## Start the stack and wait for it; the LLM is checked, never started
	@$(COMPOSE) up -d --wait
	$(MAKE) --no-print-directory health LLM_STRICT=0

.PHONY: legacy-src
legacy-src: ## Export the as-is sources of the `legacy` profile from LEGACY_SRC_REF
	@# The frozen narrative-orchestrator and semantic-memory do not compile
	@# against the current shared/ (F-2 removed go.work, F-4a replaced the bus,
	@# F-3 moved shared/{config,minio,oracle,spatial} out of the module), so the
	@# profile builds from the last commit at which they did — see
	@# build/legacy.Dockerfile. Only the paths the build needs are exported: the
	@# .mcp.env of that commit is deliberately left behind.
	rm -rf build/.legacy-src
	mkdir -p build/.legacy-src
	git archive $(LEGACY_SRC_REF) 		go.work go.mod go.sum shared 		services/narrative-orchestrator services/semantic-memory |
		tar -x -C build/.legacy-src
	echo "legacy-src: build/.legacy-src <- $(LEGACY_SRC_REF)"

.PHONY: down
down: ## Stop the containers; volumes and the LLM process are untouched
	@$(COMPOSE) down

.PHONY: reset
reset: ## Stop and DELETE the volumes; asks, and wants a fresh backup
	@latest=$$(ls -t "$(BACKUP_DIR)"/minio-*.tgz 2>/dev/null | head -n 1 || true)
	if [ -z "$$latest" ]; then
		echo "reset: no backup in $(BACKUP_DIR); run 'make backup' first" >&2
		exit 1
	fi
	echo "reset: newest backup is $$latest"
	read -r -p "Delete every volume of $(PROJECT_NAME)? type 'yes': " answer
	[ "$$answer" = yes ] || { echo "reset: cancelled"; exit 1; }
	$(COMPOSE) down -v

.PHONY: logs
logs: ## Follow the log of one service: make logs SERVICE=core
	@$(COMPOSE) logs -f --tail=200 $(SERVICE)

.PHONY: health
health: ## /health of every process plus the LLM and the age of the backup
	@rc=0
	printf '%-16s %s\n' SERVICE STATUS
	for probe in "gateway http://127.0.0.1:8088/health" \
		"memory http://127.0.0.1:8082/health" \
		"telegram-bot http://127.0.0.1:8089/health"; do
		set -- $$probe
		if code=$$(curl -s -o /dev/null -w '%{http_code}' --max-time 5 "$$2" 2>/dev/null) && [ "$$code" = 200 ]; then
			printf '%-16s ok\n' "$$1"
		elif $(COMPOSE) ps --services 2>/dev/null | grep -qx "$$1"; then
			printf '%-16s FAIL (%s)\n' "$$1" "$${code:-unreachable}"
			rc=1
		else
			printf '%-16s -\n' "$$1"
		fi
	done
	# The admin port of core is deliberately not published (T-14): probe it
	# from inside its own container.
	#
	# MSYS_NO_PATHCONV=1 is not decoration. Under Git Bash the argument
	# /multiverse is an absolute path inside the container, but MSYS rewrites it
	# to a Windows path before docker ever sees it, and the probe dies with
	# `stat C:/Program Files/Git/multiverse: no such file`. The service is fine;
	# only the probe is broken, so `make up` reported FAIL on a healthy core.
	# The variable is unknown to Linux shells and ignored there (T-403).
	if MSYS_NO_PATHCONV=1 $(COMPOSE) exec -T core /multiverse health --url http://127.0.0.1:8090/health >/dev/null 2>&1; then
		printf '%-16s ok\n' core
	else
		printf '%-16s FAIL\n' core
		rc=1
	fi
	if ! $(MAKE) --no-print-directory llm-health; then
		if [ "$(LLM_STRICT)" = 1 ]; then
			rc=1
		else
			# §6.3: an unreachable LLM degrades the narrative, it does not fail
			# the stack, so `make up` stays at zero. The wording is the one the
			# design prescribes.
			# In English, and not because the design prescribed the Russian
			# wording — it did. The Windows console renders the UTF-8 of this
			# file as CP1251, so the operator saw mojibake exactly where the
			# message mattered. The Russian wording lives in the runbook, where
			# it renders (T-403).
			echo "warning: the LLM is unreachable, the narrative degrades (FR-080); start it with: make llm-up" >&2
		fi
	fi
	latest=$$(ls -t "$(BACKUP_DIR)"/minio-*.tgz 2>/dev/null | head -n 1 || true)
	if [ -n "$$latest" ]; then
		printf '%-16s %s\n' backup "$$(basename "$$latest") ($$(( ( $$(date +%s) - $$(date -r "$$latest" +%s) ) / 86400 )) days old)"
	else
		printf '%-16s none in %s\n' backup "$(BACKUP_DIR)"
	fi
	exit $$rc

# -----------------------------------------------------------------------------
# LLM runtime (native process, outside compose)
# -----------------------------------------------------------------------------
.PHONY: llm-up
llm-up: ## Start llama-server and wait for /health=200; a second call is a no-op
	@$(call llm,up)

.PHONY: llm-down
llm-down: ## Stop llama-server by ops/llm-server.pid
	@$(call llm,down)

.PHONY: llm-health
llm-health: ## /health, /v1/models, the build pin and VRAM; non-zero unless 200
	@$(call llm,health)

.PHONY: models
models: ## Ollama only: pull the models of ops/models.txt
	@$(load_dotenv)
	if [ "$${MV_LLM_PROVIDER:-openai_compat}" != ollama ]; then
		echo "models: MV_LLM_PROVIDER is not ollama — llama.cpp models are .gguf files in MV_LLM_MODELS_DIR (ops/models.txt, section gguf)"
		exit 0
	fi
	grep -vE '^\s*(#|$$)' ops/models.txt | while read -r model; do ollama pull "$$model"; done

.PHONY: warm
warm: ## Ollama only: keep the models resident
	@$(load_dotenv)
	if [ "$${MV_LLM_PROVIDER:-openai_compat}" != ollama ]; then
		echo "warm: llama-server keeps its weights resident (mmap+mlock); make llm-up already did the first call"
		exit 0
	fi
	grep -vE '^\s*(#|$$)' ops/models.txt | while read -r model; do
		curl -s -o /dev/null -X POST "$${MV_OLLAMA_URL:-http://127.0.0.1:11434}/api/generate" \
			-d "{\"model\":\"$$model\",\"prompt\":\"\",\"keep_alive\":-1}"
	done

.PHONY: bench
bench: ## The measurement matrix (§6.4); needs a running llama-server
	@if command -v pwsh >/dev/null 2>&1; then pwsh scripts/llm-bench.ps1; else bash scripts/llm-bench.sh; fi

# -----------------------------------------------------------------------------
# Data
# -----------------------------------------------------------------------------
.PHONY: backup
backup: ## Tar the MinIO and Redpanda volumes into $(BACKUP_DIR)
	@mkdir -p "$(BACKUP_DIR)"
	stamp=$$(date +%Y%m%d-%H%M%S)
	project=$${COMPOSE_PROJECT_NAME:-multiverse}
	$(COMPOSE) stop core
	# prompts-* is excluded on purpose: it holds player texts and has its own
	# 30 day lifecycle (SEC-22, §5.6).
	docker run --rm -v "$${project}_minio-data:/src:ro" -v "$(BACKUP_DIR):/dst" $(ALPINE_IMAGE) \
		tar czf "/dst/minio-$$stamp.tgz" --exclude='prompts-*' -C /src .
	$(COMPOSE) stop redpanda
	docker run --rm -v "$${project}_redpanda-data:/src:ro" -v "$(BACKUP_DIR):/dst" $(ALPINE_IMAGE) \
		tar czf "/dst/redpanda-$$stamp.tgz" -C /src .
	$(COMPOSE) start redpanda core
	(cd "$(BACKUP_DIR)" && sha256sum "minio-$$stamp.tgz" "redpanda-$$stamp.tgz" >> SHA256SUMS)
	echo "backup: $(BACKUP_DIR)/minio-$$stamp.tgz, $(BACKUP_DIR)/redpanda-$$stamp.tgz"

.PHONY: restore
restore: ## Unpack a backup into its volume: make restore FILE=minio-<date>.tgz
	@[ -n "$(FILE)" ] || { echo "restore: pass FILE=<archive>" >&2; exit 1; }
	src=$$(cd "$$(dirname "$(FILE)")" && pwd)/$$(basename "$(FILE)")
	[ -f "$$src" ] || { echo "restore: $$src does not exist" >&2; exit 1; }
	project=$${COMPOSE_PROJECT_NAME:-multiverse}
	case "$$(basename "$$src")" in
		minio-*) volume="$${project}_minio-data" ;;
		redpanda-*) volume="$${project}_redpanda-data" ;;
		*) echo "restore: cannot tell which volume $$src belongs to" >&2; exit 1 ;;
	esac
	read -r -p "Overwrite volume $$volume from $$src? type 'yes': " answer
	[ "$$answer" = yes ] || { echo "restore: cancelled"; exit 1; }
	$(COMPOSE) down
	docker run --rm -v "$$volume:/dst" -v "$$(dirname "$$src"):/src:ro" $(ALPINE_IMAGE) \
		sh -c 'rm -rf /dst/..?* /dst/.[!.]* /dst/*; tar xzf "/src/$$(basename "'"$$src"'")" -C /dst'
	echo "restore: $$volume restored; run make up"

.PHONY: archive-legacy
archive-legacy: ## One-off: copy the as-is volumes before they are recreated (§5.5)
	@stamp=$$(date +%Y%m%d)
	dst="backups/legacy-$$stamp"
	mkdir -p "$$dst"
	project=$${COMPOSE_PROJECT_NAME:-multiverse}
	$(COMPOSE) down
	# The as-is volume names, with underscores: T-004 renamed the target ones
	# precisely so that these are never mounted by accident (D-8).
	for volume in minio_data redpanda_data; do
		if docker volume inspect "$${project}_$$volume" >/dev/null 2>&1; then
			docker run --rm -v "$${project}_$$volume:/src:ro" -v "$$PWD/$$dst:/dst" $(ALPINE_IMAGE) \
				tar czf "/dst/$${volume%_data}-as-is.tgz" -C /src .
			echo "archive-legacy: $$dst/$${volume%_data}-as-is.tgz"
		else
			echo "archive-legacy: volume $${project}_$$volume does not exist — nothing to archive"
		fi
	done
	(cd "$$dst" && sha256sum ./*.tgz > SHA256SUMS 2>/dev/null || true)

.PHONY: replay
replay: ## Replay a recording in one process: make replay RECORDING=<file>
	@[ -n "$(RECORDING)" ] || { echo "replay: pass RECORDING=<file>" >&2; exit 1; }
	go run ./cmd/multiverse --contexts=all --mode=replay --bus=memory --recording=$(RECORDING)

# -----------------------------------------------------------------------------
# Release
# -----------------------------------------------------------------------------
.PHONY: deploy
deploy: ci backup image ## Full check, backup, image, then up with the sha tag
	@if [ -f .env ] && grep -q '^MV_IMAGE_TAG=' .env; then
		grep '^MV_IMAGE_TAG=' .env > .env.previous
	fi
	MV_IMAGE_TAG=$(GIT_SHA) $(COMPOSE) up -d --wait
	$(MAKE) --no-print-directory health LLM_STRICT=0

.PHONY: rollback
rollback: ## Bring the previous image tag back (§8)
	@[ -f .env.previous ] || { echo "rollback: no .env.previous — nothing to roll back to" >&2; exit 1; }
	tag=$$(sed -n 's/^MV_IMAGE_TAG=//p' .env.previous)
	[ -n "$$tag" ] || { echo "rollback: .env.previous has no MV_IMAGE_TAG" >&2; exit 1; }
	echo "rollback: MV_IMAGE_TAG=$$tag"
	MV_IMAGE_TAG=$$tag $(COMPOSE) up -d --wait
	$(MAKE) --no-print-directory health LLM_STRICT=0

# -----------------------------------------------------------------------------
# Housekeeping
# -----------------------------------------------------------------------------
.PHONY: clean
clean: ## Remove build artefacts; no `docker system prune` (it ate other images)
	@rm -rf bin/ coverage.out

.PHONY: help
help: ## This list
	@echo "$(PROJECT_NAME) — infrastructure.md §2.2"
	echo
	grep -hE '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) |
		sort |
		awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-18s\033[0m %s\n", $$1, $$2}'
	echo
	echo "Variables: PROFILES=memory,bot  SERVICE=core  FILE=  RECORDING=  ROUTER=1  BASE=$(BASE)  PARITY_ARGS='-run U0 -v'"
