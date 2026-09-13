package providers

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	"multiverse-core.io/internal/llm"
)

type stubProvider struct{ name string }

func (s stubProvider) Generate(context.Context, llm.Request) (llm.Response, error) {
	return llm.Response{Provider: s.name}, nil
}
func (stubProvider) Embed(context.Context, string, []string) ([][]float32, error) { return nil, nil }
func (stubProvider) Health(context.Context) llm.Status                            { return llm.StatusOK }
func (stubProvider) Models(context.Context) ([]string, error)                     { return nil, nil }

// counting returns a factory that remembers whether and with what it ran.
func counting(name string, calls *int, seen *llm.Config) Factory {
	return func(cfg llm.Config) (llm.Provider, error) {
		*calls++
		*seen = cfg
		return stubProvider{name: name}, nil
	}
}

func localConfig() llm.Config {
	return llm.Config{URL: "http://127.0.0.1:8888"}
}

// The package works without a single implementation: the process registry of
// a test binary that imports no provider is empty, and New says so.
func TestTheRegistryWorksWithoutImplementations(t *testing.T) {
	if names := Names(); len(names) != 0 {
		t.Fatalf("process registry = %v, want empty: no provider package is imported", names)
	}
	_, err := New(llm.ProviderOpenAICompat, localConfig())
	if !errors.Is(err, ErrNotBuiltIn) || !strings.Contains(err.Error(), "none") {
		t.Fatalf("New on an empty registry: err = %v, want ErrNotBuiltIn listing none", err)
	}
}

func TestNewRefusesAnUnknownName(t *testing.T) {
	r := NewRegistry()
	var calls int
	var seen llm.Config
	r.Register(llm.ProviderFake, counting("fake", &calls, &seen))

	_, err := r.New("unknown", localConfig())
	if !errors.Is(err, ErrUnknownProvider) {
		t.Fatalf(`New("unknown"): err = %v, want ErrUnknownProvider`, err)
	}
	for _, want := range []string{`"unknown"`, "MV_LLM_PROVIDER", "openai_compat", "fake"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf(`New("unknown"): error %q does not mention %q`, err, want)
		}
	}
	if _, err := r.New("", localConfig()); !errors.Is(err, ErrUnknownProvider) {
		t.Errorf(`New(""): err = %v, want ErrUnknownProvider`, err)
	}
	if calls != 0 {
		t.Errorf("a factory ran for an unknown name")
	}
}

// openai and deepseek are URLs and keys of openai_compat, not provider names
// (ADR-005 add. 2 p. 2): the error says what to write instead.
func TestNewExplainsTheRetiredNames(t *testing.T) {
	r := NewRegistry()
	for _, name := range []string{"openai", "deepseek"} {
		_, err := r.New(name, localConfig())
		if !errors.Is(err, ErrUnknownProvider) || !errors.Is(err, llm.ErrConfig) {
			t.Fatalf("New(%q): err = %v, want ErrUnknownProvider and llm.ErrConfig", name, err)
		}
		for _, want := range []string{"no longer a provider name", "openai_compat", "MV_LLM_URL", "MV_LLM_API_KEY"} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("New(%q): error %q does not mention %q", name, err, want)
			}
		}
	}
}

func TestNewRefusesAValueWithoutImplementation(t *testing.T) {
	r := NewRegistry()
	var calls int
	var seen llm.Config
	r.Register(llm.ProviderFake, counting("fake", &calls, &seen))
	r.Register(llm.ProviderOpenAICompat, counting("openai_compat", &calls, &seen))

	_, err := r.New(llm.ProviderOllama, localConfig())
	if !errors.Is(err, ErrNotBuiltIn) || errors.Is(err, ErrUnknownProvider) {
		t.Fatalf("New(ollama) unregistered: err = %v, want ErrNotBuiltIn only", err)
	}
	if !strings.Contains(err.Error(), "fake, openai_compat") {
		t.Errorf("error %q does not list the built-in providers", err)
	}
}

// Switching MV_LLM_PROVIDER changes the factory and nothing else (US-014).
func TestNewBuildsTheNamedProvider(t *testing.T) {
	r := NewRegistry()
	var fakeCalls, compatCalls int
	var fakeSeen, compatSeen llm.Config
	r.Register(llm.ProviderFake, counting("fake", &fakeCalls, &fakeSeen))
	r.Register(llm.ProviderOpenAICompat, counting("openai_compat", &compatCalls, &compatSeen))

	for _, name := range []string{llm.ProviderOpenAICompat, llm.ProviderFake} {
		p, err := r.New(name, localConfig())
		if err != nil {
			t.Fatalf("New(%s): %v", name, err)
		}
		resp, _ := p.Generate(context.Background(), llm.Request{})
		if resp.Provider != name {
			t.Errorf("New(%s) built %q", name, resp.Provider)
		}
	}
	if fakeCalls != 1 || compatCalls != 1 {
		t.Errorf("factory calls fake=%d openai_compat=%d, want 1 each", fakeCalls, compatCalls)
	}
	if compatSeen.Provider != llm.ProviderOpenAICompat || compatSeen.URL != "http://127.0.0.1:8888" {
		t.Errorf("the factory got %+v, want the configuration with its provider name filled in", compatSeen)
	}
	if got := r.Names(); !slices.Equal(got, []string{"fake", "openai_compat"}) {
		t.Errorf("Names = %v", got)
	}
}

// A non-local host without MV_LLM_CLOUD_ENABLED=true: the provider does not
// start, and its factory never runs (ADR-005 add. 2 p. 3).
func TestNewStopsACloudEndpointAtTheGate(t *testing.T) {
	r := NewRegistry()
	var calls int
	var seen llm.Config
	r.Register(llm.ProviderOpenAICompat, counting("openai_compat", &calls, &seen))
	r.Register(llm.ProviderFake, counting("fake", &calls, &seen))

	cloud := llm.Config{URL: "https://api.example.com", APIKey: llm.NewSecret("zzzz-apikey-for-tests")}
	_, err := r.New(llm.ProviderOpenAICompat, cloud)
	if !errors.Is(err, llm.ErrCloudDisabled) || !errors.Is(err, llm.ErrConfig) {
		t.Fatalf("cloud endpoint without the flag: err = %v, want llm.ErrCloudDisabled", err)
	}
	if calls != 0 {
		t.Fatal("the factory ran past the cloud gate")
	}
	if strings.Contains(err.Error(), "apikey-for-tests") {
		t.Errorf("the gate error prints the key: %v", err)
	}

	cloud.Cloud.Enabled = true
	if _, err := r.New(llm.ProviderOpenAICompat, cloud); err != nil || calls != 1 {
		t.Fatalf("cloud endpoint with the flag: err = %v, factory calls %d", err, calls)
	}

	for _, u := range []string{"http://127.0.0.1:8888", "http://host.docker.internal:8888", "http://10.1.1.1:8888"} {
		if _, err := r.New(llm.ProviderOpenAICompat, llm.Config{URL: u}); err != nil {
			t.Errorf("local %s without the flag: %v", u, err)
		}
	}
	if _, err := r.New(llm.ProviderFake, llm.Config{URL: "https://api.example.com"}); err != nil {
		t.Errorf("fake does not use the URL and must start: %v", err)
	}
}

func TestNewRefusesAConfigurationOfAnotherProvider(t *testing.T) {
	r := NewRegistry()
	var calls int
	var seen llm.Config
	r.Register(llm.ProviderFake, counting("fake", &calls, &seen))
	cfg := localConfig()
	cfg.Provider = llm.ProviderOpenAICompat
	if _, err := r.New(llm.ProviderFake, cfg); err == nil || calls != 0 {
		t.Fatalf("New(fake) with an openai_compat configuration: err = %v, factory calls %d", err, calls)
	}
}

func TestNewWrapsFactoryFailures(t *testing.T) {
	r := NewRegistry()
	boom := errors.New("boom")
	r.Register(llm.ProviderFake, func(llm.Config) (llm.Provider, error) { return nil, boom })
	r.Register(llm.ProviderRecorded, func(llm.Config) (llm.Provider, error) { return nil, nil })

	if _, err := r.New(llm.ProviderFake, localConfig()); !errors.Is(err, boom) || !strings.Contains(err.Error(), "fake") {
		t.Errorf("factory error: err = %v, want boom wrapped with the provider name", err)
	}
	if p, err := r.New(llm.ProviderRecorded, localConfig()); err == nil || p != nil {
		t.Errorf("factory returned no provider: p = %v, err = %v", p, err)
	}
}

func TestRegisterPanicsOnProgrammingErrors(t *testing.T) {
	ok := func(llm.Config) (llm.Provider, error) { return stubProvider{}, nil }
	for name, register := range map[string]func(r *Registry){
		"unknown name": func(r *Registry) { r.Register("gpt", ok) },
		"retired name": func(r *Registry) { r.Register("openai", ok) },
		"nil factory":  func(r *Registry) { r.Register(llm.ProviderFake, nil) },
		"twice": func(r *Registry) {
			r.Register(llm.ProviderFake, ok)
			r.Register(llm.ProviderFake, ok)
		},
	} {
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("%s: Register did not panic", name)
				}
			}()
			register(NewRegistry())
		}()
	}
}

// Iteration 2 (review Mi-2): New does not trust a Config assembled by hand.
// A provider that needs an address gets one that LoadConfig would accept, or
// its factory does not run.
func TestNewRefusesAHandMadeConfigLoadConfigWouldRefuse(t *testing.T) {
	r := NewRegistry()
	var calls int
	var seen llm.Config
	r.Register(llm.ProviderOpenAICompat, counting("openai_compat", &calls, &seen))
	r.Register(llm.ProviderOllama, counting("ollama", &calls, &seen))

	for name, tc := range map[string]struct {
		provider string
		cfg      llm.Config
	}{
		"empty config":          {llm.ProviderOpenAICompat, llm.Config{}},
		"ollama without url":    {llm.ProviderOllama, llm.Config{URL: "http://127.0.0.1:8888"}},
		"credentials":           {llm.ProviderOpenAICompat, llm.Config{URL: "http://user:zzzz@127.0.0.1:8888"}},
		"credentials in cloud":  {llm.ProviderOpenAICompat, llm.Config{URL: "https://user:zzzz@api.example.com", Cloud: llm.Cloud{Enabled: true}}},
		"query":                 {llm.ProviderOpenAICompat, llm.Config{URL: "http://127.0.0.1:8888/?x=1"}},
		"empty query":           {llm.ProviderOpenAICompat, llm.Config{URL: "http://127.0.0.1:8888?"}},
		"fragment":              {llm.ProviderOpenAICompat, llm.Config{URL: "http://127.0.0.1:8888#f"}},
		"loopback without port": {llm.ProviderOpenAICompat, llm.Config{URL: "http://127.0.0.1"}},
	} {
		_, err := r.New(tc.provider, tc.cfg)
		if !errors.Is(err, llm.ErrConfig) {
			t.Errorf("%s: err = %v, want llm.ErrConfig", name, err)
		}
	}
	if calls != 0 {
		t.Fatalf("a factory ran %d times for a configuration LoadConfig would refuse", calls)
	}
}
