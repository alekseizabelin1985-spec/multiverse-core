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
// services on its own network. A variable with no sensible default carries an
// empty one and is marked Required or RequiredWhen.
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
	Mode = Declare("MV_MODE", "live",
		"live drives the process from the bus, replay from a recorded journal",
		OneOf("live", "replay"))
	Bus = Declare("MV_BUS", "redpanda",
		"event bus transport; memory is for e2e and for debugging a single process",
		OneOf("redpanda", "memory"))
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

	GatewayAddr = Declare("MV_GATEWAY_ADDR", ":8088",
		"listen address of the player API")
	GatewayDataDir = Declare("MV_GATEWAY_DATA_DIR", "/data",
		"directory holding links.db and gateway.db")
	GatewayClientIDs = Declare("MV_GATEWAY_CLIENT_IDS", "telegram-bot,ci-harness,mvctl",
		"comma separated allow-list of X-Client-Id (ADR-009 p. 9); prod drops ci-harness")
	GatewayActorKindClients = Declare("MV_GATEWAY_ACTOR_KIND_CLIENTS", "ci-harness,mvctl",
		"clients allowed to send X-Actor-Kind ci or sim")
	CoreURL = Declare("MV_CORE_URL", "http://127.0.0.1:8090",
		"core address the gateway proxies /v1/admin/* to (D-7)")

	// --- core -------------------------------------------------------------

	CoreAddr = Declare("MV_CORE_ADDR", "127.0.0.1:8090",
		"listen address of /health and /v1/admin/* of the process (D-7)")
	CoreAdminClients = Declare("MV_CORE_ADMIN_CLIENTS", "operator,mvctl",
		"comma separated X-Client-Id admitted to /v1/admin/* (ADR-009 p. 9); "+
			"the default is the short list the platform ships with, and mvctl is "+
			"on it in advance: no subcommand of this build calls the routes yet, "+
			"its world, laws and snapshot subcommands arrive in EPIC-002/003 "+
			"(decision ОВ-28)")
	MemoryURL = Declare("MV_MEMORY_URL", "",
		"memory service address; empty switches memory off (degradation FR-035)")
	SnapshotEveryFacts = Declare("MV_SNAPSHOT_EVERY_FACTS", "200",
		"state writes a snapshot every N facts", IsInt())
	GMPath = Declare("MV_GM_PATH", "agent",
		"game master path; the feature flag of the migration off the legacy orchestrator (S5)",
		OneOf("agent", "legacy"))
	LawsBreachPhase = Declare("MV_LAWS_BREACH_PHASE", "false",
		"enable the law breach phase (ADR-008); off for MVP-1", IsBool())

	// --- llm (the default runtime is the native llama-server, ADR-005 add. 2) ---

	LLMProvider = Declare("MV_LLM_PROVIDER", "openai_compat",
		"LLM provider implementation (C-15 v1.1); openai in the cloud is this one with another URL and key",
		OneOf("openai_compat", "ollama", "anthropic", "recorded", "fake"))
	LLMURL = Declare("MV_LLM_URL", "http://127.0.0.1:1234",
		"LLM endpoint; from a container http://host.docker.internal:1234")
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
	LLMHost = Declare("MV_LLM_HOST", "127.0.0.1",
		"interface llama-server binds; never published outside the host (SEC-15)", Tooling())
	LLMPort = Declare("MV_LLM_PORT", "1234",
		"port llama-server binds", Tooling(), IsInt())
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

	MemoryAddr = Declare("MV_MEMORY_ADDR", ":8082",
		"listen address of the memory service")
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
}
