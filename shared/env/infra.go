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
var (
	ComposeProjectName = DeclareExternal("COMPOSE_PROJECT_NAME", "multiverse",
		"docker compose project name", Tooling())
	ComposeProfiles = DeclareExternal("COMPOSE_PROFILES", "memory,bot",
		"compose profiles to raise: memory, gpu, bot, dev, legacy", Tooling())
	// Compose takes COMPOSE_ENV_FILES from the environment of its own process
	// and never from an env file: the variable names the env files, so it
	// cannot come out of one of them (T-412, checked on compose v5.2). The
	// line in .env is therefore inert. It stays declared because it documents
	// the one value the Makefile exports — the only way the stack sees
	// build/versions.env — and a shell exports for a manual compose command.
	ComposeEnvFiles = DeclareExternal("COMPOSE_ENV_FILES", ".env,build/versions.env",
		"env files docker compose reads, taken ONLY from the environment of the "+
			"compose process — the Makefile exports it, a manual command needs it "+
			"exported in the shell; the line in .env is inert (T-412). Image "+
			"versions come from build/versions.env only", Tooling())

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

	// The OLLAMA_* block carries the settings of the stand as defaults
	// (contracts.md §16 p. 5, v0.7; infrastructure.md §6.4 and §6.5, the
	// loopback origins of SEC-15). No platform process reads them, so for the
	// platform these defaults are a document; the value reaches the container
	// through compose, ${OLLAMA_X:-<the default below>}, and rule 8 of
	// scripts/compose-lint.sh holds compose to exactly these values. A key with
	// no value would not do: it hands the container the image's own default,
	// not ours. For a native Ollama they are a recommendation — the server
	// reads the environment of the OS, not .env.
	//
	// They used to be declared empty and RequiredWhen(MV_LLM_PROVIDER=ollama),
	// while compose supplied its own defaults: two sources of one value, and a
	// requirement nothing could fail, since compose always filled the gap
	// (T-411, decided in T-416, done in T-413). MV_OLLAMA_URL keeps its
	// conditional requirement — an address cannot be guessed.
	OllamaKeepAlive = DeclareExternal("OLLAMA_KEEP_ALIVE", "-1",
		"how long Ollama keeps a model loaded; -1 keeps it forever (infrastructure.md §6.4)",
		Tooling())
	OllamaMaxLoadedModels = DeclareExternal("OLLAMA_MAX_LOADED_MODELS", "2",
		"models Ollama keeps in VRAM at once (infrastructure.md §6.4)",
		Tooling(), IsInt())
	OllamaNumParallel = DeclareExternal("OLLAMA_NUM_PARALLEL", "1",
		"requests Ollama serves in parallel (infrastructure.md §6.4)",
		Tooling(), IsInt())
	OllamaFlashAttention = DeclareExternal("OLLAMA_FLASH_ATTENTION", "1",
		"enable flash attention in Ollama (infrastructure.md §6.4)",
		Tooling())
	OllamaKVCacheType = DeclareExternal("OLLAMA_KV_CACHE_TYPE", "f16",
		"KV cache precision of Ollama, f16 or q8_0 (infrastructure.md §6.4)",
		Tooling())
	OllamaOrigins = DeclareExternal("OLLAMA_ORIGINS", "http://127.0.0.1,http://localhost",
		"allowed CORS origins of Ollama, loopback only; never `*` (SEC-15)",
		Tooling())
)
