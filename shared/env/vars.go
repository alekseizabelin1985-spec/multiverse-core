package env

// The manifest of the platform variables (infrastructure.md v0.3 §4.2, the
// source of truth for the set and the names).
//
// The declarations live here rather than next to the packages that read them
// for one reason: mvctl env check must see the whole manifest whatever the
// binary happens to import. A declaration in internal/gateway would be invisible
// to a check run from mvctl, and the comparison with .env.example would pass
// while missing half the variables. A context still reads its variable through
// its own name here, never through os.Getenv.
//
// Defaults are the values of a process started on the host with `go run`
// (infrastructure.md §4.3); compose overrides them with the names of the
// services on its own network — for the six network addresses of
// contracts.md §16 p. 5 only (MV_KAFKA_BROKERS, MV_MINIO_ENDPOINT, MV_CORE_URL,
// MV_QDRANT_ADDR, MV_NEO4J_URI, MV_TELEGRAM_GATEWAY_URL), and in the shape of
// the default declared here; rule 8 of scripts/compose-lint.sh holds every
// other compose default to the one below (T-411, T-413). A variable with no
// sensible default carries an empty one and is marked Required or
// RequiredWhen.
//
// A variable that is read as a number, a flag or a duration also declares its
// kind, so that a value which will not parse is reported by mvctl env check
// instead of by the first typed read inside a running process. The budget in
// MV_LLM_CLOUD_BUDGET_USD_PER_DAY is deliberately left untyped: it is an
// amount of money and may carry a fraction, and the manifest has no such kind.
var (
	// --- compose -------------------------------------------------------

	// ImageTag is read by docker compose, not by a process.
	ImageTag = Declare("MV_IMAGE_TAG", "dev",
		"tag of the multiverse-core image; make deploy substitutes the git sha", Tooling())

	// --- platform, common to every process ------------------------------

	Environment = Declare("MV_ENV", "dev",
		"deployment this process runs in", OneOf("dev", "ci", "prod"))
	LogLevel = Declare("MV_LOG_LEVEL", "info",
		"slog level; debug is never enabled in prod (infrastructure.md §7.1)",
		OneOf("debug", "info", "warn", "error"))
	LogFormat = Declare("MV_LOG_FORMAT", "json",
		"log encoding; the platform ships json, text is for a local run",
		OneOf("json", "text"))
	// Mode and Bus are the source of truth for the two choices cmd/multiverse
	// makes at start; --mode and --bus take their DEFAULT from here and only
	// override it when a developer passes them explicitly (T-408, the shape of
	// T-404). Until T-408 both were declared here, shipped into every container
	// by compose and read by NOBODY: the choice was made by flags compose never
	// passed, so an operator who wrote replay in .env got live and was told
	// nothing. The enums below are also the only dictionary of accepted values
	// — serve.go builds its check out of Enum() instead of repeating the list,
	// which is what stopped the two from drifting apart in the first place.
	Mode = Declare("MV_MODE", "live",
		"live drives the process from the bus, replay from a recorded journal; "+
			"--mode overrides it for a run started by hand",
		OneOf("live", "replay"))
	// The value names the PROTOCOL, not the product implementing it. The
	// manifest used to say redpanda while the flag said kafka, and the two sets
	// did not intersect — whichever one an operator wrote, the other half of
	// the platform had no name for it. kafka is the survivor, and not by taste:
	// the brokers are already configured in MV_KAFKA_BROKERS, the client is a
	// Kafka client (shared/eventbus/kafka.go), and Redpanda is one
	// implementation of that API among several — moving to Kafka proper, MSK or
	// Warpstream must not require renaming a value that describes something
	// which did not change. The retired value is not accepted as a silent
	// synonym: Validate refuses it here and serve.go refuses it with the
	// sentence that names the replacement (T-408).
	Bus = Declare("MV_BUS", "kafka",
		"event bus transport: kafka is the Kafka API (Redpanda in compose, "+
			"brokers in MV_KAFKA_BROKERS), memory is the in-process bus for "+
			"e2e and for debugging a single process and needs --contexts=all; "+
			"--bus overrides it for a run started by hand",
		OneOf("kafka", "memory"))
	BusValidateOnRead = Declare("MV_BUS_VALIDATE_ON_READ", "true",
		"validate an event against its schema when reading it (ADR-007 add. 4); "+
			"the process passes eventbus.Delivery.SkipValidateOnRead = !this", IsBool())
	KafkaBrokers = Declare("MV_KAFKA_BROKERS", "127.0.0.1:19092",
		"comma separated bus brokers; inside compose redpanda:9092, on the host the external listener")
	MinIOEndpoint = Declare("MV_MINIO_ENDPOINT", "127.0.0.1:9000",
		"object store endpoint, host:port without a scheme")
	MinIOAccessKey = Declare("MV_MINIO_ACCESS_KEY", "",
		"object store access key; in dev the MinIO root user, in prod a service user",
		Secret(), Required())
	MinIOSecretKey = Declare("MV_MINIO_SECRET_KEY", "",
		"object store secret key", Secret(), Required())
	MinIOUseSSL = Declare("MV_MINIO_USE_SSL", "false",
		"talk to the object store over https", IsBool())
	WorldID = Declare("MV_WORLD_ID", "dark-forest-world",
		"world used by mvctl and by the bootstrap when none is given")
	BackupAgeRecipient = Declare("MV_BACKUP_AGE_RECIPIENT", "",
		"public age key the links.db backup is encrypted to; a public key, not a secret")

	// --- gateway ---------------------------------------------------------

	// There is no MV_GATEWAY_ADDR: one process serves one HTTP server, and its
	// address is MV_CORE_ADDR whatever the --contexts set is (see below, T-408).
	GatewayDataDir = Declare("MV_GATEWAY_DATA_DIR", "/data",
		"directory holding links.db and gateway.db")
	GatewayClientIDs = Declare("MV_GATEWAY_CLIENT_IDS", "telegram-bot,ci-harness,mvctl",
		"comma separated allow-list of X-Client-Id (ADR-009 p. 9); prod drops ci-harness")
	GatewayActorKindClients = Declare("MV_GATEWAY_ACTOR_KIND_CLIENTS", "ci-harness,mvctl",
		"clients allowed to send X-Actor-Kind ci or sim")
	CoreURL = Declare("MV_CORE_URL", "http://127.0.0.1:8090",
		"core address the gateway proxies /v1/admin/* to (D-7)")

	// --- core -------------------------------------------------------------

	// The single listen address of a process, whichever contexts it runs.
	// serve.go creates ONE runtime.NewHTTP and hands its mux to every context
	// of --contexts, so /health, /v1/admin/* and — in a process that runs the
	// gateway context — the player API are all behind this one address; with
	// --contexts=all they are behind it in one process. MV_GATEWAY_ADDR and
	// MV_MEMORY_ADDR were two more names for that same one address, read by
	// nobody, and compose had to copy their value into this variable for
	// anything to happen at all. Worse, they looked adjustable and were not:
	// the published port, the healthcheck and MV_CORE_URL of the gateway are
	// literals, so changing the "listen address" alone only broke the stack
	// (T-408). Both names are declared retired in init below.
	CoreAddr = Declare("MV_CORE_ADDR", "127.0.0.1:8090",
		"listen address of the HTTP server of THIS process — /health, "+
			"/v1/admin/* (D-7) and the player API when the process runs the "+
			"gateway context. Inside compose each service carries the address "+
			"its published port and its healthcheck agree on; this value is "+
			"what a process started by hand on the host listens on")
	CoreAdminClients = Declare("MV_CORE_ADMIN_CLIENTS", "operator,mvctl,ci-harness",
		"comma separated X-Client-Id admitted to /v1/admin/* (ADR-009 p. 9); "+
			"the default is the short list the platform ships with, and mvctl "+
			"and ci-harness are on it in advance: no subcommand of this build "+
			"calls the routes yet, its world, laws and snapshot subcommands "+
			"arrive in EPIC-002/003, and ci-harness is the client id the e2e "+
			"scenarios of CI use from T-018 on (decision ОВ-28)")
	MemoryURL = Declare("MV_MEMORY_URL", "",
		"memory service address; empty switches memory off (degradation FR-035)")
	SnapshotEveryFacts = Declare("MV_SNAPSHOT_EVERY_FACTS", "200",
		"state writes a snapshot every N facts", IsInt())
	// StateWorlds is read by internal/state when the context starts: one worker
	// per world, the single writer of that world (EPIC-002 design.md §4.3, T-055).
	StateWorlds = Declare("MV_STATE_WORLDS", "dark-forest-world",
		"comma separated worlds the context state serves, one worker each; "+
			"proposals of any other world are passed over")
	GMPath = Declare("MV_GM_PATH", "agent",
		"game master path; the feature flag of the migration off the legacy orchestrator (S5)",
		OneOf("agent", "legacy"))
	LawsBreachPhase = Declare("MV_LAWS_BREACH_PHASE", "false",
		"enable the law breach phase (ADR-008); off for MVP-1", IsBool())
	// SwarmFake belongs to the process, not to a context: cmd/multiverse reads
	// it when it builds the context swarm, because at I1-α there is no swarm
	// context to read it in (ADR-001 addendum p. 8). It leaves with the hook
	// that reads it, in T-256.
	SwarmFake = Declare("MV_SWARM_FAKE", "false",
		"run the context swarm as the Phase 1 stub of I1-α (shared/testkit/swarm: "+
			"FakeEncounter and FakeNarrator) instead of the swarm; read by "+
			"cmd/multiverse/fake_contexts.go, a temporary hook removed by T-256", IsBool())

	// --- llm (the default runtime is the native llama-server, ADR-005 add. 2) ---

	LLMProvider = Declare("MV_LLM_PROVIDER", "openai_compat",
		"LLM provider implementation (C-15 v1.1); openai in the cloud is this one with another URL and key",
		OneOf("openai_compat", "ollama", "anthropic", "recorded", "fake"))
	// The value may be written with or without a trailing /v1 — vendors hand
	// out the address both ways, and scripts/lib/llm-endpoint.sh and
	// scripts/lib/LlmEndpoint.psm1 reduce it to the base address themselves
	// and say so, rather than sending the operator to edit the value by hand
	// (owner's decision, journal 2026-09-11). The path under the base is
	// appended by whoever calls, so a value carrying /v1 must not be appended
	// to twice.
	//
	// REQUIRED, with no default. It used to declare http://127.0.0.1:1234, and
	// that made the one case the task exists to close — the line deleted from
	// .env — have TWO answers: the platform silently talked to 1234 while
	// make llm-health said there was no address (T-404 review #2 M-4,
	// orchestrator decision). An LLM address cannot be guessed; a quiet default
	// pointing at a port anything on this machine may hold is exactly the trap
	// the task started from.
	LLMURL = Declare("MV_LLM_URL", "",
		"LLM endpoint, the base address of an OpenAI-compatible server; a "+
			"trailing /v1 is accepted and trimmed. From a container "+
			"http://host.docker.internal:<port>. Required and without a "+
			"default: the single source of truth for the address AND for the "+
			"rule that reads it — scripts/lib/llm-endpoint.* derive the port "+
			"llama-server binds, the address make llm-health probes and the "+
			"address make bench measures from this value, and never from a "+
			"second variable (T-404)", Required())
	LLMAPIKey = Declare("MV_LLM_API_KEY", "",
		"LLM key; empty for the local llama-server, set only for a cloud endpoint", Secret())
	LLMNumCtx = Declare("MV_LLM_NUM_CTX", "8192",
		"context window per slot; --ctx-size of llama-server is this times the number of slots",
		IsInt())
	LLMStorePrompts = Declare("MV_LLM_STORE_PROMPTS", "false",
		"store full prompts in prompts-{world} (ILM 30 days, SEC-22)", IsBool())
	LLMCloudEnabled = Declare("MV_LLM_CLOUD_ENABLED", "false",
		"allow a non-local host in MV_LLM_URL; the gate is the address, not the provider name (ADR-005 add. 2 p. 3)",
		IsBool())
	LLMCloudAllowExternalPlayers = Declare("MV_LLM_CLOUD_ALLOW_EXTERNAL_PLAYERS", "false",
		"allow prompts of external players to reach a cloud endpoint", IsBool())
	LLMCloudBudgetUSDPerDay = Declare("MV_LLM_CLOUD_BUDGET_USD_PER_DAY", "0",
		"daily budget for cloud calls in USD; 0 means no cloud spending is allowed")
	AnthropicAPIKey = Declare("MV_ANTHROPIC_API_KEY", "",
		"key of the anthropic provider (E-H)",
		Secret(), RequiredWhen("MV_LLM_PROVIDER", "anthropic"))

	// --- llama-server: read by scripts/llm-server.* only -------------------
	//
	// These never reach a container, so a process must not fail when they are
	// unset: they are declared to stay visible to mvctl env check and to
	// .env.example, with scope tooling (infrastructure.md v0.3 §4.2 p. 5).

	LLMBin = Declare("MV_LLM_BIN", "",
		"path to llama-server on the host", Tooling())
	LLMModelFile = Declare("MV_LLM_MODEL_FILE", "",
		"model file for the single model mode of llama-server (-m), a .gguf", Tooling())
	LLMModelsDir = Declare("MV_LLM_MODELS_DIR", "",
		"model directory for the router mode of llama-server (--models-dir, without -m)", Tooling())
	LLMSlotSavePath = Declare("MV_LLM_SLOT_SAVE_PATH", "",
		"directory llama-server saves slot state to", Tooling())
	// LLMHost is not a duplicate of the host in MV_LLM_URL: one says what the
	// local server listens on, the other says where a client knocks, and they
	// are allowed to differ (a container knocks at host.docker.internal while
	// the server binds loopback). The port was the duplicate and is gone: it is
	// read out of MV_LLM_URL (T-404).
	LLMHost = Declare("MV_LLM_HOST", "127.0.0.1",
		"interface llama-server binds; loopback only, never published outside the "+
			"host (SEC-15) — a non-loopback value makes make llm-up refuse to "+
			"start, because llama-server has no authentication in front of it. "+
			"The port it binds is not declared here — it comes from MV_LLM_URL",
		Tooling())
	LLMSlots = Declare("MV_LLM_SLOTS", "1",
		"--parallel of llama-server", Tooling(), IsInt())
	LLMNGL = Declare("MV_LLM_NGL", "99",
		"--n-gpu-layers of llama-server", Tooling(), IsInt())
	LLMThreads = Declare("MV_LLM_THREADS", "32",
		"--threads of llama-server", Tooling(), IsInt())
	LLMBatchSize = Declare("MV_LLM_BATCH_SIZE", "16000",
		"--batch-size of llama-server", Tooling(), IsInt())
	LLMReasoning = Declare("MV_LLM_REASONING", "off",
		"--reasoning of llama-server; the gateway also sends enable_thinking=false per request",
		Tooling(), OneOf("on", "off", "auto"))

	// --- ollama (only with MV_LLM_PROVIDER=ollama) -------------------------

	OllamaURL = Declare("MV_OLLAMA_URL", "",
		"Ollama endpoint; needed only with the ollama provider (ADR-005 add. 2 p. 2)",
		RequiredWhen("MV_LLM_PROVIDER", "ollama"))

	// --- memory (compose profile memory) -----------------------------------

	// There is no MV_MEMORY_ADDR either: the memory process listens on
	// MV_CORE_ADDR like every other one (T-408).
	QdrantAddr = Declare("MV_QDRANT_ADDR", "127.0.0.1:6334",
		"Qdrant gRPC address")
	Neo4jURI = Declare("MV_NEO4J_URI", "neo4j://127.0.0.1:7687",
		"Neo4j bolt URI")
	Neo4jUser = Declare("MV_NEO4J_USER", "neo4j",
		"Neo4j user")
	Neo4jPassword = Declare("MV_NEO4J_PASSWORD", "",
		"Neo4j password; equal to NEO4J_PASSWORD of the image", Secret())
	EmbedModel = Declare("MV_EMBED_MODEL", "nomic-embed-text",
		"embedding model of the memory service")

	// --- telegram-bot (compose profile bot) ---------------------------------

	TelegramBotToken = Declare("MV_TELEGRAM_BOT_TOKEN", "",
		"bot token from @BotFather; passed to the telegram-bot service only", Secret())
	TelegramAllowedUserIDs = Declare("MV_TELEGRAM_ALLOWED_USER_IDS", "",
		"comma separated allow-list of Telegram user ids (SEC-06); empty means the bot admits nobody")
	TelegramGatewayURL = Declare("MV_TELEGRAM_GATEWAY_URL", "http://127.0.0.1:8088",
		"gateway address the bot calls")
	TelegramPollTimeoutS = Declare("MV_TELEGRAM_POLL_TIMEOUT_S", "25",
		"long polling timeout of getUpdates, in seconds", IsInt())
	TelegramHealthAddr = Declare("MV_TELEGRAM_HEALTH_ADDR", ":8089",
		"listen address of /health of the bot")
)

func init() {
	// openai and deepseek stopped being provider names in ADR-005 add. 2:
	// they are the openai_compat provider with another URL and key. The names
	// stay here so that mvctl env check tells an operator whose .env still
	// carries them what to do, instead of silently ignoring them.
	DeclareDeprecated("MV_OPENAI_API_KEY",
		"removed in infrastructure.md v0.3: use MV_LLM_URL and MV_LLM_API_KEY with MV_LLM_PROVIDER=openai_compat")
	DeclareDeprecated("MV_DEEPSEEK_API_KEY",
		"removed in infrastructure.md v0.3: use MV_LLM_URL and MV_LLM_API_KEY with MV_LLM_PROVIDER=openai_compat")

	// The address had two sources of truth: the platform read MV_LLM_URL while
	// the scripts assembled their own from MV_LLM_HOST and MV_LLM_PORT. They
	// drifted on the owner's stand — the server listened on 8888, MV_LLM_URL
	// said 8888, MV_LLM_PORT still said 1234, and make llm-health reported a
	// dead LLM while it was serving. The port is now derived from MV_LLM_URL;
	// the name stays declared retired so that an operator whose .env still
	// carries it is told to delete the line instead of being ignored (T-404).
	DeclareDeprecated("MV_LLM_PORT",
		"the port is part of MV_LLM_URL, which is the only source of the address (T-404); "+
			"MV_LLM_HOST stays and means the interface the local server binds")

	// One process serves one HTTP server (see MV_CORE_ADDR): these two were a
	// second and a third name for its address, declared, shipped by compose and
	// read by no code at all — the compose file copied their value into
	// MV_CORE_ADDR, which is what a process really reads. They are retired
	// rather than deleted quietly so that an operator whose .env still carries
	// a line is told which variable took it over, instead of watching a setting
	// disappear without a word (T-408, the rule of T-404).
	DeclareDeprecated("MV_GATEWAY_ADDR",
		"one process serves one HTTP server and its address is MV_CORE_ADDR (T-408); "+
			"in compose the gateway carries MV_CORE_ADDR=:8088, the port it publishes")
	DeclareDeprecated("MV_MEMORY_ADDR",
		"one process serves one HTTP server and its address is MV_CORE_ADDR (T-408); "+
			"in compose the memory service carries MV_CORE_ADDR=:8082, the port it publishes")
}
