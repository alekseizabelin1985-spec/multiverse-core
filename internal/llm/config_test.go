package llm

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"testing"
	"time"

	"gopkg.in/yaml.v3"

	"multiverse-core.io/shared/env"
)

func source(kv ...string) env.Source {
	m := make(map[string]string, len(kv)/2)
	for i := 0; i+1 < len(kv); i += 2 {
		m[kv[i]] = kv[i+1]
	}
	return env.MapSource(m)
}

func TestLoadConfigDefaults(t *testing.T) {
	cfg, err := LoadConfig(source("MV_LLM_URL", "http://127.0.0.1:8888"))
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	want := Config{
		Provider: ProviderOpenAICompat,
		URL:      "http://127.0.0.1:8888",
		NumCtx:   8192,
		Timeouts: Timeouts{
			Narrative: 20 * time.Second,
			Tick:      30 * time.Second,
			Decision:  5 * time.Second,
			Degraded:  3 * time.Second,
		},
		PricesPath: "config/llm-prices.yaml",
	}
	if cfg != want {
		t.Fatalf("LoadConfig defaults =\n%+v\nwant\n%+v", cfg, want)
	}
}

func TestLoadConfigReadsEveryVariable(t *testing.T) {
	cfg, err := LoadConfig(source(
		"MV_LLM_PROVIDER", "openai_compat",
		"MV_LLM_URL", "https://api.example.com/v1",
		"MV_LLM_API_KEY", "k",
		"MV_LLM_NUM_CTX", "16384",
		"MV_LLM_STORE_PROMPTS", "true",
		"MV_LLM_CLOUD_ENABLED", "true",
		"MV_LLM_CLOUD_ALLOW_EXTERNAL_PLAYERS", "true",
		"MV_LLM_CLOUD_BUDGET_USD_PER_DAY", "1.25",
		"MV_LLM_TIMEOUT_NARRATIVE", "7s",
		"MV_LLM_TIMEOUT_TICK", "11s",
		"MV_LLM_TIMEOUT_DECISION", "1500ms",
		"MV_LLM_TIMEOUT_DEGRADED", "900ms",
		"MV_LLM_PRICES", "/etc/prices.yaml",
	))
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if !cfg.APIKey.IsSet() || cfg.APIKey.Reveal() != "k" {
		t.Fatalf("APIKey = %q (set %v), want k", cfg.APIKey.Reveal(), cfg.APIKey.IsSet())
	}
	want := Config{
		Provider:     ProviderOpenAICompat,
		URL:          "https://api.example.com",
		APIKey:       cfg.APIKey, // a Secret compares by identity; its value is checked above
		NumCtx:       16384,
		StorePrompts: true,
		Cloud:        Cloud{Enabled: true, AllowExternalPlayers: true, BudgetUSDPerDay: 1.25},
		Timeouts: Timeouts{
			Narrative: 7 * time.Second,
			Tick:      11 * time.Second,
			Decision:  1500 * time.Millisecond,
			Degraded:  900 * time.Millisecond,
		},
		PricesPath: "/etc/prices.yaml",
	}
	if cfg != want {
		t.Fatalf("LoadConfig =\n%+v\nwant\n%+v", cfg, want)
	}
}

// The value is accepted with and without /v1 and reduced to the base address
// (C-15, T-404); a path the proxy needs is kept.
func TestLoadConfigTrimsTheV1Tail(t *testing.T) {
	for raw, want := range map[string]string{
		"http://127.0.0.1:8888":            "http://127.0.0.1:8888",
		"http://127.0.0.1:8888/":           "http://127.0.0.1:8888",
		"http://127.0.0.1:8888/v1":         "http://127.0.0.1:8888",
		"http://127.0.0.1:8888/v1/":        "http://127.0.0.1:8888",
		" http://host.docker.internal:1 ":  "http://host.docker.internal:1",
		"https://api.example.com/v1":       "https://api.example.com",
		"http://10.0.0.5/proxy/llm/v1":     "http://10.0.0.5/proxy/llm",
		"http://10.0.0.5/v10":              "http://10.0.0.5/v10",
		"http://[::1]:8080/v1":             "http://[::1]:8080",
		"http://192.168.1.2:8888/prefixv1": "http://192.168.1.2:8888/prefixv1",
	} {
		cfg, err := LoadConfig(source("MV_LLM_URL", raw))
		if err != nil {
			t.Errorf("MV_LLM_URL=%q: %v", raw, err)
			continue
		}
		if cfg.URL != want {
			t.Errorf("MV_LLM_URL=%q: URL = %q, want %q", raw, cfg.URL, want)
		}
	}
}

// MV_LLM_URL has no default (T-404): a provider that talks to it refuses to
// load without it, a provider that does not leave the process loads.
func TestLoadConfigRequiresTheURLOnlyForProvidersThatUseIt(t *testing.T) {
	for _, p := range []string{ProviderOpenAICompat, ProviderAnthropic} {
		_, err := LoadConfig(source("MV_LLM_PROVIDER", p))
		if !errors.Is(err, ErrConfig) || !strings.Contains(err.Error(), "MV_LLM_URL") {
			t.Errorf("provider %s without MV_LLM_URL: err = %v, want a configuration error naming MV_LLM_URL", p, err)
		}
	}
	for _, p := range []string{ProviderFake, ProviderRecorded} {
		cfg, err := LoadConfig(source("MV_LLM_PROVIDER", p))
		if err != nil {
			t.Errorf("provider %s without MV_LLM_URL: %v", p, err)
		}
		if name, addr := cfg.Endpoint(); name != "" || addr != "" {
			t.Errorf("provider %s: Endpoint = (%q, %q), want none", p, name, addr)
		}
	}
}

func TestLoadConfigRefusesABadURLWithoutEchoingIt(t *testing.T) {
	for _, raw := range []string{
		"127.0.0.1:8888",
		"ftp://127.0.0.1/v1",
		"http://",
		"http://user:hunter2pass@127.0.0.1:8888",
		"http://127.0.0.1:8888/v1?key=hunter2pass",
		"http://127.0.0.1:8888/#hunter2pass",
		"http://127.0.0.1:8888?",
		"http://127.0.0.1:8888/v1?",
		"http://127.0.0.1:8888/#",
		"http://127.0.0.1:8888#",
	} {
		_, err := LoadConfig(source("MV_LLM_URL", raw))
		if !errors.Is(err, ErrConfig) {
			t.Errorf("MV_LLM_URL=%q: err = %v, want ErrConfig", raw, err)
			continue
		}
		if strings.Contains(err.Error(), "hunter2") {
			t.Errorf("MV_LLM_URL=%q: the error repeats a credential: %v", raw, err)
		}
	}
}

// openai and deepseek are an address and a key of openai_compat now
// (ADR-005 add. 2): the error says which edit fixes .env.
func TestLoadConfigExplainsRetiredAndUnknownProviders(t *testing.T) {
	for _, name := range []string{"openai", "deepseek"} {
		_, err := LoadConfig(source("MV_LLM_PROVIDER", name, "MV_LLM_URL", "http://127.0.0.1:1"))
		if !errors.Is(err, ErrConfig) {
			t.Fatalf("MV_LLM_PROVIDER=%s: err = %v, want ErrConfig", name, err)
		}
		for _, want := range []string{name, "openai_compat", "MV_LLM_URL", "MV_LLM_API_KEY"} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("MV_LLM_PROVIDER=%s: error %q does not mention %q", name, err, want)
			}
		}
	}
	_, err := LoadConfig(source("MV_LLM_PROVIDER", "gpt4all", "MV_LLM_URL", "http://127.0.0.1:1"))
	if !errors.Is(err, ErrConfig) {
		t.Fatalf("unknown provider: err = %v, want ErrConfig", err)
	}
	for _, want := range env.LLMProvider.Enum() {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("unknown provider: error %q does not list %q", err, want)
		}
	}
}

func TestProviderConstantsAreTheManifestEnum(t *testing.T) {
	got := []string{ProviderOpenAICompat, ProviderOllama, ProviderAnthropic, ProviderRecorded, ProviderFake}
	want := env.LLMProvider.Enum()
	slices.Sort(got)
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Fatalf("provider constants %v, MV_LLM_PROVIDER enum %v", got, want)
	}
}

// MV_OLLAMA_URL is declared for everyone and read by the ollama provider only.
func TestLoadConfigReadsTheOllamaURLOnlyForOllama(t *testing.T) {
	cfg, err := LoadConfig(source("MV_LLM_URL", "http://127.0.0.1:1", "MV_OLLAMA_URL", "not a url"))
	if err != nil {
		t.Fatalf("openai_compat with a garbage MV_OLLAMA_URL: %v", err)
	}
	if cfg.OllamaURL != "" {
		t.Fatalf("openai_compat read MV_OLLAMA_URL: %q", cfg.OllamaURL)
	}

	_, err = LoadConfig(source("MV_LLM_PROVIDER", "ollama"))
	if !errors.Is(err, ErrConfig) || !strings.Contains(err.Error(), "MV_OLLAMA_URL") {
		t.Fatalf("ollama without MV_OLLAMA_URL: err = %v", err)
	}
	cfg, err = LoadConfig(source("MV_LLM_PROVIDER", "ollama", "MV_OLLAMA_URL", "http://127.0.0.1:11434/"))
	if err != nil {
		t.Fatalf("ollama: %v", err)
	}
	if name, addr := cfg.Endpoint(); name != "MV_OLLAMA_URL" || addr != "http://127.0.0.1:11434" {
		t.Fatalf("ollama: Endpoint = (%q, %q)", name, addr)
	}
}

func TestLoadConfigRefusesBadValues(t *testing.T) {
	for _, tc := range []struct{ name, value string }{
		{"MV_LLM_NUM_CTX", "0"},
		{"MV_LLM_NUM_CTX", "big"},
		{"MV_LLM_STORE_PROMPTS", "maybe"},
		{"MV_LLM_CLOUD_ENABLED", "yes please"},
		{"MV_LLM_CLOUD_ALLOW_EXTERNAL_PLAYERS", "2"},
		{"MV_LLM_CLOUD_BUDGET_USD_PER_DAY", "-1"},
		{"MV_LLM_CLOUD_BUDGET_USD_PER_DAY", "NaN"},
		{"MV_LLM_CLOUD_BUDGET_USD_PER_DAY", "ten"},
		{"MV_LLM_TIMEOUT_NARRATIVE", "20"},
		{"MV_LLM_TIMEOUT_TICK", "0s"},
		{"MV_LLM_TIMEOUT_DECISION", "-5s"},
		{"MV_LLM_TIMEOUT_DEGRADED", ""},
		{"MV_LLM_PRICES", ""},
	} {
		_, err := LoadConfig(source("MV_LLM_URL", "http://127.0.0.1:1", tc.name, tc.value))
		if err == nil || !strings.Contains(err.Error(), tc.name) {
			t.Errorf("%s=%q: err = %v, want an error naming the variable", tc.name, tc.value, err)
		}
	}
}

func TestLoadConfigReportsEveryProblemAtOnce(t *testing.T) {
	_, err := LoadConfig(source("MV_LLM_URL", "", "MV_LLM_NUM_CTX", "0", "MV_LLM_TIMEOUT_TICK", "x"))
	for _, want := range []string{"MV_LLM_URL", "MV_LLM_NUM_CTX", "MV_LLM_TIMEOUT_TICK"} {
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("err = %v, want it to name %s", err, want)
		}
	}
}

func TestTimeoutsFor(t *testing.T) {
	tm := Timeouts{Narrative: 1, Tick: 2, Decision: 3, Degraded: 4}
	for p, want := range map[Phase]time.Duration{PhaseNarrative: 1, PhaseTick: 2, PhaseDecision: 3} {
		if got, err := tm.For(p); err != nil || got != want {
			t.Errorf("For(%s) = %v, %v; want %v", p, got, err, want)
		}
	}
	for _, p := range []Phase{PhaseOther, "", "chat"} {
		if _, err := tm.For(p); err == nil {
			t.Errorf("For(%q) has no error", p)
		}
	}
}

func TestIsLocalEndpoint(t *testing.T) {
	local := []string{
		"http://127.0.0.1:8888", "http://127.10.0.1", "http://localhost:1234/v1", "http://LOCALHOST.:1",
		"http://[::1]:8080", "http://[::ffff:127.0.0.1]:1", "http://host.docker.internal:8888",
		"http://[::ffff:10.0.0.1]:1", "http://[::ffff:192.168.0.1]:1", "http://[::ffff:172.20.0.1]:1",
		"http://10.0.0.1", "http://10.255.255.255", "http://172.16.0.1", "http://172.31.255.255",
		"https://192.168.0.10:443/v1",
	}
	remote := []string{
		"https://api.openai.com/v1", "https://api.deepseek.com", "http://8.8.8.8",
		"http://172.15.255.255", "http://172.32.0.1", "http://11.0.0.1", "http://192.169.0.1",
		"http://169.254.169.254", "http://0.0.0.0:8888", "http://[fd00::1]:1", "http://[2001:db8::1]",
		"http://ollama:11434", "http://10.0.0.1.example.com", "http://localhost.example.com",
		"http://host.docker.internal.evil.com",
	}
	for _, u := range local {
		if ok, err := IsLocalEndpoint(u); err != nil || !ok {
			t.Errorf("IsLocalEndpoint(%q) = %v, %v; want true", u, ok, err)
		}
	}
	for _, u := range remote {
		if ok, err := IsLocalEndpoint(u); err != nil || ok {
			t.Errorf("IsLocalEndpoint(%q) = %v, %v; want false", u, ok, err)
		}
	}
	for _, u := range []string{"", "127.0.0.1:8888", "ftp://127.0.0.1", "http://"} {
		if _, err := IsLocalEndpoint(u); err == nil {
			t.Errorf("IsLocalEndpoint(%q) has no error", u)
		}
	}
}

// The gate is the address, not the provider name (ADR-005 add. 2 p. 3).
func TestCheckCloudGate(t *testing.T) {
	load := func(kv ...string) Config {
		t.Helper()
		cfg, err := LoadConfig(source(kv...))
		if err != nil {
			t.Fatalf("LoadConfig(%v): %v", kv, err)
		}
		return cfg
	}

	for _, u := range []string{"http://127.0.0.1:8888", "http://host.docker.internal:8888/v1", "http://192.168.1.5:1"} {
		if err := load("MV_LLM_URL", u).CheckCloudGate(); err != nil {
			t.Errorf("local %s without the cloud flag: %v", u, err)
		}
	}

	cloud := load("MV_LLM_URL", "https://api.example.com/v1")
	err := cloud.CheckCloudGate()
	if !errors.Is(err, ErrCloudDisabled) || !errors.Is(err, ErrConfig) {
		t.Fatalf("cloud URL without MV_LLM_CLOUD_ENABLED: err = %v, want ErrCloudDisabled and ErrConfig", err)
	}
	for _, want := range []string{"MV_LLM_URL", "api.example.com", "MV_LLM_CLOUD_ENABLED=true"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("gate error %q does not mention %q", err, want)
		}
	}

	if err := load("MV_LLM_URL", "https://api.example.com/v1", "MV_LLM_CLOUD_ENABLED", "true").CheckCloudGate(); err != nil {
		t.Errorf("cloud URL with MV_LLM_CLOUD_ENABLED=true: %v", err)
	}
	if err := load("MV_LLM_PROVIDER", "anthropic", "MV_LLM_URL", "https://api.anthropic.example").CheckCloudGate(); !errors.Is(err, ErrCloudDisabled) {
		t.Errorf("anthropic without the cloud flag: err = %v, want ErrCloudDisabled", err)
	}
	if err := load("MV_LLM_PROVIDER", "fake", "MV_LLM_URL", "https://api.example.com").CheckCloudGate(); err != nil {
		t.Errorf("fake does not leave the process, the gate must not stop it: %v", err)
	}
	err = load("MV_LLM_PROVIDER", "ollama", "MV_OLLAMA_URL", "https://ollama.example.com").CheckCloudGate()
	if !errors.Is(err, ErrCloudDisabled) || !strings.Contains(err.Error(), "MV_OLLAMA_URL") {
		t.Errorf("ollama on a remote host: err = %v, want ErrCloudDisabled naming MV_OLLAMA_URL", err)
	}
}

// The key and its fragments reach no output a Config can be printed to
// (SEC-21, ADR-009 p. 4).
//
// Iteration 2 (review Mi-1): String alone left two holes — a verb that does
// not fit a string (%d) made fmt print the value through reflection, and
// yaml.Marshal printed it as a plain string. Every verb of fmt is tried on the
// key alone and on the key nested in a Config, a pointer, a struct, a slice
// and a map.
func TestTheKeyIsNeverPrinted(t *testing.T) {
	const key = "zzzz-apikey-yyyy-for-tests-xxxx"
	cfg, err := LoadConfig(source(
		"MV_LLM_URL", "https://api.example.com/v1",
		"MV_LLM_API_KEY", key,
	))
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.APIKey.Reveal() != key {
		t.Fatalf("Reveal = %q, the key was lost", cfg.APIKey.Reveal())
	}

	type holder struct {
		Name string
		LLM  Config
		Ptr  *Config
		Key  Secret
	}
	values := map[string]any{
		"key":           cfg.APIKey,
		"*key":          &cfg.APIKey,
		"config":        cfg,
		"*config":       &cfg,
		"nested":        holder{Name: "n", LLM: cfg, Ptr: &cfg, Key: cfg.APIKey},
		"slice":         []Secret{cfg.APIKey},
		"map":           map[string]Config{"llm": cfg},
		"anonymous":     struct{ K Secret }{cfg.APIKey},
		"interface key": any(cfg.APIKey),
	}
	verbs := []string{
		"%v", "%+v", "%#v", "%s", "%q", "%x", "%X", "%d", "%b", "%o", "%O", "%c", "%U",
		"%e", "%E", "%f", "%F", "%g", "%G", "%t", "%p", "%w", "%10.3s", "%-20q", "%08x", "% x", "%!",
	}

	outputs := map[string]string{}
	for name, v := range values {
		for _, verb := range verbs {
			outputs[name+" "+verb] = fmt.Sprintf(verb, v)
		}
		outputs[name+" Sprint"] = fmt.Sprint(v)
		outputs[name+" Sprintln"] = fmt.Sprintln(v)
		outputs[name+" Errorf"] = fmt.Errorf("wrapped: %v", v).Error()

		y, err := yaml.Marshal(v)
		if err != nil {
			t.Fatalf("yaml.Marshal(%s): %v", name, err)
		}
		outputs[name+" yaml"] = string(y)
		j, err := json.Marshal(v)
		if err != nil {
			t.Fatalf("json.Marshal(%s): %v", name, err)
		}
		outputs[name+" json"] = string(j)

		var jsonLog, textLog bytes.Buffer
		slog.New(slog.NewJSONHandler(&jsonLog, nil)).Info("m", "v", v)
		slog.New(slog.NewTextHandler(&textLog, nil)).Info("m", "v", v)
		outputs[name+" slog json"] = jsonLog.String()
		outputs[name+" slog text"] = textLog.String()
	}
	outputs["gate"] = fmt.Sprint(cfg.CheckCloudGate())
	outputs["LogValue"] = cfg.LogValue().String()
	outputs["bad value"] = fmt.Sprint(LoadConfig(source("MV_LLM_URL", "http://127.0.0.1:1", "MV_LLM_API_KEY", key, "MV_LLM_NUM_CTX", "x")))

	fragments := []string{key, key[:6], key[len(key)-6:], hex.EncodeToString([]byte(key[:6])),
		strings.ToUpper(hex.EncodeToString([]byte(key[:6])))}
	for how, out := range outputs {
		for _, frag := range fragments {
			if strings.Contains(out, frag) {
				t.Errorf("%s prints the key fragment %q: %s", how, frag, out)
			}
		}
	}

	var logs bytes.Buffer
	slog.New(slog.NewJSONHandler(&logs, nil)).Info("config", "llm", cfg)
	if !strings.Contains(logs.String(), `"api_key_set":true`) {
		t.Errorf("slog does not say the key is set: %s", logs.String())
	}
	// The box alone already keeps the key out of %d; Format is what makes the
	// output say so instead of printing an address.
	for _, verb := range []string{"%d", "%x", "%t", "%v"} {
		if got := fmt.Sprintf(verb, cfg.APIKey); got != redacted {
			t.Errorf("Sprintf(%s, key) = %q, want the placeholder %q", verb, got, redacted)
		}
	}
	if got := fmt.Sprintf("%d", Secret{}); got != "" {
		t.Errorf("an empty secret prints %q, want nothing", got)
	}
	if got := outputs["config yaml"]; !strings.Contains(got, "apikey: '[redacted]'") && !strings.Contains(got, `apikey: "[redacted]"`) {
		t.Errorf("yaml of a Config does not show the placeholder: %s", got)
	}
}

// Iteration 2 (review Mi-2): the gate does not trust a Config assembled by
// hand. An address the provider needs is required, and it passes the checks
// of LoadConfig, so a factory never sees what LoadConfig would have refused.
func TestCheckCloudGateOnAHandMadeConfig(t *testing.T) {
	for _, tc := range []struct {
		name string
		cfg  Config
		want string
	}{
		{"openai_compat without an address", Config{Provider: ProviderOpenAICompat}, "MV_LLM_URL"},
		{"anthropic without an address", Config{Provider: ProviderAnthropic}, "MV_LLM_URL"},
		{"ollama without an address", Config{Provider: ProviderOllama, URL: "http://127.0.0.1:1"}, "MV_OLLAMA_URL"},
		{"credentials", Config{Provider: ProviderOpenAICompat, URL: "http://user:hunter2pass@127.0.0.1:8888"}, "credentials"},
		{"query", Config{Provider: ProviderOpenAICompat, URL: "http://127.0.0.1:8888?x=hunter2pass"}, "query"},
		{"empty query", Config{Provider: ProviderOpenAICompat, URL: "http://127.0.0.1:8888?"}, "query"},
		{"fragment", Config{Provider: ProviderOpenAICompat, URL: "http://127.0.0.1:8888#hunter2pass"}, "fragment"},
		{"not a URL", Config{Provider: ProviderOpenAICompat, URL: "127.0.0.1:8888"}, "http(s)"},
		{"no port on loopback", Config{Provider: ProviderOpenAICompat, URL: "http://127.0.0.1"}, "no port"},
		{"port out of range", Config{Provider: ProviderOllama, OllamaURL: "http://127.0.0.1:70000"}, "1-65535"},
	} {
		err := tc.cfg.CheckCloudGate()
		if !errors.Is(err, ErrConfig) || errors.Is(err, ErrCloudDisabled) {
			t.Errorf("%s: err = %v, want a configuration error that is not the cloud refusal", tc.name, err)
			continue
		}
		if !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: error %q does not mention %q", tc.name, err, tc.want)
		}
		if strings.Contains(err.Error(), "hunter2") {
			t.Errorf("%s: the error repeats a credential: %v", tc.name, err)
		}
	}
	for _, p := range []string{ProviderFake, ProviderRecorded} {
		if err := (Config{Provider: p}).CheckCloudGate(); err != nil {
			t.Errorf("%s needs no address: %v", p, err)
		}
	}
}

// Iteration 2 (review N-1, the rule of llm-endpoint.sh): a port, when written,
// is 1-65535; an address a local runtime answers on names its port.
func TestLoadConfigChecksThePort(t *testing.T) {
	for _, raw := range []string{
		"http://127.0.0.1", "http://127.0.0.1/v1", "http://localhost", "https://LOCALHOST./v1",
		"http://host.docker.internal", "http://[::1]/v1", "http://0.0.0.0", "http://[::]",
		"http://127.0.0.1:", "http://127.0.0.1:0", "http://127.0.0.1:65536", "http://127.0.0.1:99999",
		"https://api.example.com:0/v1", "http://10.0.0.5:123456", "https://api.example.com:/v1",
	} {
		if _, err := LoadConfig(source("MV_LLM_URL", raw)); !errors.Is(err, ErrConfig) {
			t.Errorf("MV_LLM_URL=%q: err = %v, want ErrConfig", raw, err)
		}
	}
	for _, raw := range []string{
		"http://127.0.0.1:1", "http://127.0.0.1:65535", "http://[::1]:8080",
		"https://api.example.com/v1", "http://api.example.com", "http://192.168.1.2/v1", "http://10.0.0.5",
	} {
		if _, err := LoadConfig(source("MV_LLM_URL", raw)); err != nil {
			t.Errorf("MV_LLM_URL=%q: %v", raw, err)
		}
	}
}

// Iteration 2 (review N-2): one spelling of an IP host for the gate and the
// price table.
func TestEndpointHostIsCanonical(t *testing.T) {
	for raw, want := range map[string]string{
		"http://[2001:DB8:0::1]:8080/v1":   "2001:db8::1",
		"http://[::FFFF:10.0.0.1]:1":       "10.0.0.1",
		"https://API.Example.COM./v1":      "api.example.com",
		"http://[0:0:0:0:0:0:0:1]:8888/v1": "::1",
	} {
		if got, err := EndpointHost(raw); err != nil || got != want {
			t.Errorf("EndpointHost(%q) = %q, %v; want %q", raw, got, err, want)
		}
	}
}
