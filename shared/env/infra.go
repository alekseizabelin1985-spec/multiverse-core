package env

// The variables of the third-party images and of docker compose. Their names
// are dictated by whoever reads them, so they carry no MV_ prefix and are
// declared with DeclareExternal (infrastructure.md v0.3 §4.1 p. 2). This file
// is the list mvctl env check compares the non-prefixed part of .env.example
// against (§4.2).
//
// None of them is read by a platform process, so all carry scope tooling: a
// core running in a container never sees MINIO_ROOT_PASSWORD and must not fail
// because of it. Compose enforces its own requirements with ${VAR:?message}.
// The exception is the conditional requirement of the OLLAMA_* block, which
// holds wherever the variables are read: with MV_LLM_PROVIDER=ollama they must
// be set, with any other provider their absence is not an error (ADR-005
// add. 2 p. 2).
var (
	ComposeProjectName = DeclareExternal("COMPOSE_PROJECT_NAME", "multiverse",
		"docker compose project name", Tooling())
	ComposeProfiles = DeclareExternal("COMPOSE_PROFILES", "memory,bot",
		"compose profiles to raise: memory, gpu, bot, dev, legacy", Tooling())
	ComposeEnvFiles = DeclareExternal("COMPOSE_ENV_FILES", ".env,build/versions.env",
		"env files compose reads; image versions come from build/versions.env only", Tooling())

	// The three below are marked Required because infrastructure.md §4.2 says
	// so; the flag is what lets mvctl env check report an environment that has
	// none of them. It costs a process nothing: validate drops the
	// unconditional requirement of a tooling variable, so a core in a
	// container still starts without ever seeing MINIO_ROOT_PASSWORD. The
	// requirement of NEO4J_PASSWORD really holds only under the memory
	// profile, which RequiredWhen cannot express — COMPOSE_PROFILES is a list
	// and a condition compares the whole value — so it is recorded
	// unconditionally and compose keeps enforcing it with ${VAR:?}.
	MinIORootUser = DeclareExternal("MINIO_ROOT_USER", "",
		"MinIO root user; required by compose and never minioadmin", Tooling(), Required())
	MinIORootPassword = DeclareExternal("MINIO_ROOT_PASSWORD", "",
		"MinIO root password, at least 16 characters; required by compose",
		Secret(), Tooling(), Required())
	Neo4jImagePassword = DeclareExternal("NEO4J_PASSWORD", "",
		"Neo4j password; compose builds NEO4J_AUTH=neo4j/${NEO4J_PASSWORD}, required by the memory profile",
		Secret(), Tooling(), Required())

	OllamaKeepAlive = DeclareExternal("OLLAMA_KEEP_ALIVE", "",
		"how long Ollama keeps a model loaded; -1 keeps it forever",
		Tooling(), RequiredWhen("MV_LLM_PROVIDER", "ollama"))
	OllamaMaxLoadedModels = DeclareExternal("OLLAMA_MAX_LOADED_MODELS", "",
		"models Ollama keeps in VRAM at once",
		Tooling(), IsInt(), RequiredWhen("MV_LLM_PROVIDER", "ollama"))
	OllamaNumParallel = DeclareExternal("OLLAMA_NUM_PARALLEL", "",
		"requests Ollama serves in parallel",
		Tooling(), IsInt(), RequiredWhen("MV_LLM_PROVIDER", "ollama"))
	OllamaFlashAttention = DeclareExternal("OLLAMA_FLASH_ATTENTION", "",
		"enable flash attention in Ollama",
		Tooling(), RequiredWhen("MV_LLM_PROVIDER", "ollama"))
	OllamaKVCacheType = DeclareExternal("OLLAMA_KV_CACHE_TYPE", "",
		"KV cache precision of Ollama, f16 or q8_0 (infrastructure.md §6.4)",
		Tooling(), RequiredWhen("MV_LLM_PROVIDER", "ollama"))
)
