# =============================================================================
# Makefile — the operator's entry point (infrastructure.md v0.3 §2.2)
# =============================================================================
# One target, one action. Rules of this file:
#   * `docker compose`, never `docker-compose` (v1);
#   * no image tag is ever written into a recipe — every version comes from
#     build/versions.env, which is included below (NFR-071);
#   * GNU make 4.4 with bash: `winget install ezwinports.make` on Windows, or
#     `wsl make <target>` without installing anything (§2.1);
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

# Compose reads .env and build/versions.env. .env declares COMPOSE_ENV_FILES
# for a bare `docker compose`; this default keeps `make` working when it does
# not (§4.2).
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
BASE ?= integration/mvp-1

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

.PHONY: test-integration
test-integration: minio-image ## testcontainers: Redpanda, MinIO, Qdrant, Neo4j
	@go test -tags integration -count=1 -timeout 15m ./...

.PHONY: test-e2e
test-e2e: ## End to end in one process, in-memory bus
	@go test -tags e2e -count=1 -timeout 10m ./...

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
	@gitleaks git --no-banner --redact --log-opts="$(BASE)..HEAD" .
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
	# A rule that quietly stopped firing looks exactly like a clean file, so
	# every negative fixture has to stay rejected (testdata/compose-lint).
	for bad in testdata/compose-lint/bad-*.yml; do
		if scripts/compose-lint.sh -f "$$bad" >/dev/null 2>&1; then
			echo "compose-lint: $$bad broke a rule but passed the linter" >&2
			exit 1
		fi
		echo "compose-lint: $$bad rejected, as it must be"
	done

.PHONY: ci
ci: lint test contracts secrets-scan privacy-scan vuln compose-lint test-e2e ## Everything CI runs without Docker

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
	echo "Variables: PROFILES=memory,bot  SERVICE=core  FILE=  RECORDING=  ROUTER=1  BASE=$(BASE)"
