package env

import (
	"errors"
	"strings"
	"testing"
	"time"
)

// isolate swaps the package registry for an empty one, so that a test may
// declare variables of its own without adding them to the manifest the rest of
// the suite compares with .env.example. Tests using it must not run in
// parallel: the registry is package state.
func isolate(t *testing.T) {
	t.Helper()
	registry.mu.Lock()
	vars, order, deprecated := registry.vars, registry.order, registry.deprecated
	registry.vars = make(map[string]Var)
	registry.order = nil
	registry.deprecated = make(map[string]string)
	registry.mu.Unlock()
	t.Cleanup(func() {
		registry.mu.Lock()
		registry.vars, registry.order, registry.deprecated = vars, order, deprecated
		registry.mu.Unlock()
	})
}

func mustPanic(t *testing.T, want string, fn func()) {
	t.Helper()
	defer func() {
		r := recover()
		if r == nil {
			t.Fatalf("no panic, want one mentioning %q", want)
		}
		if msg, _ := r.(string); !strings.Contains(msg, want) {
			t.Fatalf("panic %q, want one mentioning %q", r, want)
		}
	}()
	fn()
}

func TestDeclareRejectsNameWithoutPlatformPrefix(t *testing.T) {
	isolate(t)
	mustPanic(t, "must start with MV_", func() {
		Declare("KAFKA_BROKERS", "", "brokers")
	})
}

func TestDeclareExternalRejectsPlatformPrefix(t *testing.T) {
	isolate(t)
	mustPanic(t, "carries the platform prefix", func() {
		DeclareExternal("MV_OLLAMA_URL", "", "ollama")
	})
}

func TestDeclareRejectsInvalidName(t *testing.T) {
	isolate(t)
	mustPanic(t, "not a valid variable name", func() {
		Declare("MV_lower_case", "", "doc")
	})
}

func TestDeclareRequiresDescription(t *testing.T) {
	isolate(t)
	mustPanic(t, "needs a description", func() {
		Declare("MV_THING", "", "  ")
	})
}

func TestDeclareRejectsDuplicate(t *testing.T) {
	isolate(t)
	Declare("MV_THING", "", "doc")
	mustPanic(t, "declared twice", func() {
		Declare("MV_THING", "", "doc")
	})
}

func TestDeclareRejectsRequiredWithDefault(t *testing.T) {
	isolate(t)
	mustPanic(t, "a default means it is not required", func() {
		Declare("MV_THING", "value", "doc", Required())
	})
}

func TestDeclareDeprecatedRejectsDeclaredName(t *testing.T) {
	isolate(t)
	Declare("MV_THING", "", "doc")
	mustPanic(t, "both declared and deprecated", func() {
		DeclareDeprecated("MV_THING", "gone")
	})
}

func TestManifestIsSortedAndCarriesFlags(t *testing.T) {
	isolate(t)
	Declare("MV_SECOND", "", "second")
	Declare("MV_FIRST", "one", "first", Secret())
	DeclareExternal("NEO4J_PASSWORD", "", "neo4j", Tooling())

	manifest := Manifest()
	var names []string
	for _, v := range manifest {
		names = append(names, v.Name())
	}
	want := []string{"MV_FIRST", "MV_SECOND", "NEO4J_PASSWORD"}
	if strings.Join(names, ",") != strings.Join(want, ",") {
		t.Fatalf("Manifest names = %v, want %v", names, want)
	}
	if !manifest[0].IsSecret() || manifest[0].Default() != "one" || manifest[0].Doc() != "first" {
		t.Fatalf("MV_FIRST lost its declaration: %+v", manifest[0])
	}
	if !manifest[2].IsExternal() || manifest[2].Scope() != ScopeTooling {
		t.Fatalf("NEO4J_PASSWORD lost its declaration: %+v", manifest[2])
	}
	if _, ok := Lookup("MV_FIRST"); !ok {
		t.Fatal("Lookup does not find a declared variable")
	}
	if _, ok := Lookup("MV_ABSENT"); ok {
		t.Fatal("Lookup finds a variable nobody declared")
	}
}

func TestStringFallsBackToDefault(t *testing.T) {
	isolate(t)
	v := Declare("MV_THING", "fallback", "doc")

	if got := v.String(); got != "fallback" {
		t.Fatalf("unset: String = %q, want the default", got)
	}
	if v.Set() {
		t.Fatal("Set reports an unset variable as set")
	}
	t.Setenv("MV_THING", "  value  ")
	if got := v.String(); got != "value" {
		t.Fatalf("set: String = %q, want the trimmed value", got)
	}
	if !v.Set() {
		t.Fatal("Set reports a set variable as unset")
	}
}

// An explicitly empty value overrides a non-empty default. The case that makes
// this a security property rather than a preference is an allow-list: an
// operator who closes /v1/admin/* by emptying MV_CORE_ADMIN_CLIENTS must not be
// handed the shipped "operator" back (review T-007 Mi-6).
func TestEmptyValueOverridesTheDefault(t *testing.T) {
	isolate(t)
	v := Declare("MV_THING", "fallback", "doc")
	list := Declare("MV_ADMIN_CLIENTS", "operator", "doc")

	t.Setenv("MV_THING", "")
	if got := v.String(); got != "" {
		t.Fatalf("empty: String = %q, want the empty value, not the default", got)
	}
	if v.Set() {
		t.Fatal("Set reports an empty variable as set")
	}
	t.Setenv("MV_THING", "   ")
	if got := v.String(); got != "" {
		t.Fatalf("blank: String = %q, want the empty value", got)
	}

	t.Setenv("MV_ADMIN_CLIENTS", "")
	if got := list.List(); len(got) != 0 {
		t.Fatalf("empty allow-list = %v, want nobody", got)
	}

	empty := MapSource(map[string]string{"MV_ADMIN_CLIENTS": ""})
	if got := list.ListFrom(empty); len(got) != 0 {
		t.Fatalf("empty allow-list from a source = %v, want nobody", got)
	}
	if got := list.ListFrom(MapSource(map[string]string{})); len(got) != 1 || got[0] != "operator" {
		t.Fatalf("unset allow-list = %v, want the default", got)
	}
}

// A required variable set to an empty value is missing, not defaulted: the
// distinction between unset and set-to-empty must not open a hole in Validate.
func TestValidateRejectsAnExplicitlyEmptyRequiredVariable(t *testing.T) {
	isolate(t)
	Declare("MV_NEEDED", "", "doc", Required())

	if err := Validate(MapSource(map[string]string{"MV_NEEDED": ""})); err == nil {
		t.Fatal("Validate accepts a required variable set to an empty value")
	}
}

// Validate parses the value the way the typed getter will, so mvctl env check
// reports it instead of the first Int/Bool/Duration call in a running process
// (review T-007 Mi-5).
func TestValidateChecksTheDeclaredKind(t *testing.T) {
	isolate(t)
	Declare("MV_FACTS", "200", "doc", IsInt())
	Declare("MV_CLOUD", "false", "doc", IsBool())
	Declare("MV_PAUSE", "25s", "doc", IsDuration())
	Declare("MV_NAME", "core", "doc")

	good := MapSource(map[string]string{
		"MV_FACTS": "10", "MV_CLOUD": "true", "MV_PAUSE": "100ms", "MV_NAME": "anything",
	})
	if err := Validate(good); err != nil {
		t.Fatalf("Validate on well typed values = %v, want nil", err)
	}

	err := Validate(MapSource(map[string]string{
		"MV_FACTS": "abc", "MV_CLOUD": "yes", "MV_PAUSE": "25",
	}))
	if err == nil {
		t.Fatal("Validate accepts values that no typed getter can read")
	}
	for _, want := range []string{
		`MV_FACTS="abc"`, "expected an integer",
		`MV_CLOUD="yes"`, "expected true or false",
		`MV_PAUSE="25"`, "expected a duration",
	} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("Validate error = %q, want it to mention %q", err, want)
		}
	}

	if err := Validate(MapSource(map[string]string{"MV_FACTS": ""})); err != nil {
		t.Fatalf("Validate on an empty optional value = %v, want nil: an unset variable has no type", err)
	}
}

// The type check must not turn into a leak: the reason is reported, the value
// of a secret is not (ADR-009 p. 4).
func TestValidateReportsATypedSecretWithoutItsValue(t *testing.T) {
	isolate(t)
	Declare("MV_BUDGET", "", "doc", Secret(), IsInt())

	err := Validate(MapSource(map[string]string{"MV_BUDGET": "not-a-number-secret"}))
	if err == nil {
		t.Fatal("Validate accepts a secret that is not an integer")
	}
	if strings.Contains(err.Error(), "not-a-number-secret") {
		t.Fatalf("Validate error carries the secret: %q", err)
	}
	if !strings.Contains(err.Error(), "withheld") {
		t.Fatalf("Validate error = %q, want a note that the value is withheld", err)
	}
}

func TestDeclareRejectsADefaultOfTheWrongKind(t *testing.T) {
	isolate(t)
	mustPanic(t, "is not a valid int", func() {
		Declare("MV_FACTS", "many", "doc", IsInt())
	})
}

// infrastructure.md §4.2 marks the MinIO credentials of the image and the Neo4j
// password as required; the manifest has to say so, or mvctl env check cannot
// check them. Marking them must not make a process fail: they are tooling
// variables and never reach a container (review T-007 Mi-4).
func TestImageCredentialsAreRequiredInTheManifestButNotForAProcess(t *testing.T) {
	for _, name := range []string{"MINIO_ROOT_USER", "MINIO_ROOT_PASSWORD", "NEO4J_PASSWORD"} {
		v, ok := Lookup(name)
		if !ok {
			t.Fatalf("%s is not declared", name)
		}
		if !v.IsRequired() {
			t.Errorf("%s is not marked required; infrastructure.md §4.2 says it is", name)
		}
		if v.Scope() != ScopeTooling {
			t.Errorf("%s has scope %q, want tooling: a container never sees it", name, v.Scope())
		}
	}

	withoutTheImageCredentials := MapSource(map[string]string{
		"MV_MINIO_ACCESS_KEY": "key", "MV_MINIO_SECRET_KEY": "secret",
	})
	if err := Validate(withoutTheImageCredentials); err != nil {
		t.Fatalf("Validate = %v, want nil: a process must not need the credentials of the image", err)
	}
}

func TestTypedGetters(t *testing.T) {
	isolate(t)
	number := Declare("MV_NUMBER", "7", "doc")
	flag := Declare("MV_FLAG", "true", "doc")
	pause := Declare("MV_PAUSE", "25s", "doc")
	list := Declare("MV_LIST", "a, b ,,c", "doc")

	if n, err := number.Int(); err != nil || n != 7 {
		t.Fatalf("Int = %d, %v; want 7, nil", n, err)
	}
	if b, err := flag.Bool(); err != nil || !b {
		t.Fatalf("Bool = %v, %v; want true, nil", b, err)
	}
	if d, err := pause.Duration(); err != nil || d != 25*time.Second {
		t.Fatalf("Duration = %v, %v; want 25s, nil", d, err)
	}
	if got := strings.Join(list.List(), "|"); got != "a|b|c" {
		t.Fatalf("List = %q, want a|b|c", got)
	}

	t.Setenv("MV_NUMBER", "many")
	if _, err := number.Int(); err == nil {
		t.Fatal("Int accepts a value that is not a number")
	} else if !strings.Contains(err.Error(), `MV_NUMBER="many"`) {
		t.Fatalf("Int error = %q, want the name and the value", err)
	}
	t.Setenv("MV_FLAG", "yes please")
	if _, err := flag.Bool(); err == nil {
		t.Fatal("Bool accepts a value that is not a boolean")
	}
	t.Setenv("MV_PAUSE", "soon")
	if _, err := pause.Duration(); err == nil {
		t.Fatal("Duration accepts a value that is not a duration")
	}
}

func TestSecretValueNeverReachesTheError(t *testing.T) {
	isolate(t)
	token := Declare("MV_TOKEN", "", "doc", Secret())
	t.Setenv("MV_TOKEN", "123456789:AAsecret-value")

	_, err := token.Int()
	if err == nil {
		t.Fatal("Int accepts a token as a number")
	}
	if strings.Contains(err.Error(), "AAsecret-value") {
		t.Fatalf("the error carries the secret: %q", err)
	}
	if !strings.Contains(err.Error(), "MV_TOKEN") || !strings.Contains(err.Error(), "withheld") {
		t.Fatalf("error = %q, want the name and a note that the value is withheld", err)
	}
	var typed *Error
	if !errors.As(err, &typed) {
		t.Fatalf("error %T is not *env.Error", err)
	}
	if typed.Value != "" {
		t.Fatalf("Error.Value = %q, want empty for a secret", typed.Value)
	}
}

func TestValidateRequiredAndEnum(t *testing.T) {
	isolate(t)
	Declare("MV_NEEDED", "", "doc", Required())
	Declare("MV_CHOICE", "one", "doc", OneOf("one", "two"))
	Declare("MV_HOST_PATH", "", "doc", Tooling(), Required())

	err := Validate(MapSource(map[string]string{"MV_CHOICE": "three"}))
	if err == nil {
		t.Fatal("Validate accepts a missing required variable and a value outside the enum")
	}
	msg := err.Error()
	for _, want := range []string{"MV_NEEDED", "required and empty", "MV_CHOICE", "one, two"} {
		if !strings.Contains(msg, want) {
			t.Fatalf("Validate error = %q, want it to mention %q", msg, want)
		}
	}
	if strings.Contains(msg, "MV_HOST_PATH") {
		t.Fatalf("Validate error = %q, want no unconditional requirement on a tooling variable", msg)
	}

	if err := Validate(MapSource(map[string]string{"MV_NEEDED": "here"})); err != nil {
		t.Fatalf("Validate on a complete environment = %v, want nil", err)
	}
}

// The requirement of the Ollama block is conditional: it holds only while the
// selected provider is ollama (ADR-005 add. 2 p. 2, tasks.md T-007).
func TestValidateOllamaBlockIsRequiredOnlyForTheOllamaProvider(t *testing.T) {
	openaiCompat := MapSource(map[string]string{
		"MV_LLM_PROVIDER":     "openai_compat",
		"MV_MINIO_ACCESS_KEY": "key",
		"MV_MINIO_SECRET_KEY": "secret",
	})
	if err := Validate(openaiCompat); err != nil {
		t.Fatalf("openai_compat without the Ollama block = %v, want nil", err)
	}

	ollama := MapSource(map[string]string{
		"MV_LLM_PROVIDER":        "ollama",
		"MV_MINIO_ACCESS_KEY":    "key",
		"MV_MINIO_SECRET_KEY":    "secret",
		"OLLAMA_KEEP_ALIVE":      "-1",
		"OLLAMA_NUM_PARALLEL":    "1",
		"OLLAMA_FLASH_ATTENTION": "1",
		"OLLAMA_KV_CACHE_TYPE":   "f16",
	})
	err := Validate(ollama)
	if err == nil {
		t.Fatal("the ollama provider without OLLAMA_MAX_LOADED_MODELS and MV_OLLAMA_URL is accepted")
	}
	for _, want := range []string{"MV_OLLAMA_URL", "OLLAMA_MAX_LOADED_MODELS", "MV_LLM_PROVIDER is ollama"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("Validate error = %q, want it to mention %q", err, want)
		}
	}
	if strings.Contains(err.Error(), "OLLAMA_KEEP_ALIVE") {
		t.Fatalf("Validate error = %q, want no complaint about a variable that is set", err)
	}
}

// The required MinIO credentials are the only unconditional requirement of the
// manifest; a process without them cannot reach the object store at all.
func TestValidateRealManifestNeedsTheObjectStoreCredentials(t *testing.T) {
	err := Validate(MapSource(map[string]string{}))
	if err == nil {
		t.Fatal("Validate accepts an empty environment")
	}
	for _, want := range []string{"MV_MINIO_ACCESS_KEY", "MV_MINIO_SECRET_KEY"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("Validate error = %q, want it to mention %q", err, want)
		}
	}
	if strings.Contains(err.Error(), "withheld") == false {
		t.Fatalf("Validate error = %q, want the credentials reported without their values", err)
	}

	full := MapSource(map[string]string{"MV_MINIO_ACCESS_KEY": "key", "MV_MINIO_SECRET_KEY": "secret"})
	if err := Validate(full); err != nil {
		t.Fatalf("Validate with the credentials set = %v, want nil", err)
	}
}

func TestDeprecatedNamesAreReportedWhenStillPresent(t *testing.T) {
	found := Deprecated(MapSource(map[string]string{
		"MV_OPENAI_API_KEY": "sk-xxxxxxxx",
		"MV_LLM_PROVIDER":   "openai_compat",
	}))
	if len(found) != 1 {
		t.Fatalf("Deprecated = %v, want one entry", found)
	}
	if !strings.Contains(found[0], "MV_OPENAI_API_KEY") || !strings.Contains(found[0], "MV_LLM_URL") {
		t.Fatalf("Deprecated = %q, want the name and what to use instead", found[0])
	}
	if strings.Contains(found[0], "sk-xxxxxxxx") {
		t.Fatalf("Deprecated = %q, want no value in the report", found[0])
	}
}
