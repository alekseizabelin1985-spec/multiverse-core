package llm

import (
	"errors"
	"math"
	"path/filepath"
	"strings"
	"testing"
)

const shippedPrices = "../../config/llm-prices.yaml"

// The model of base configuration E as llama-server serves it under --alias
// (build/versions.env LLM_MODEL_DEFAULT, T-437).
const modelE = "Qwen3.8-27B-UD-Q3_K_XL"

func TestShippedPriceTableLoadsAndLocalCallsAreFree(t *testing.T) {
	p, err := LoadPrices(filepath.FromSlash(shippedPrices))
	if err != nil {
		t.Fatalf("LoadPrices(%s): %v", shippedPrices, err)
	}
	usage := Tokens{Prompt: 420, Completion: 130, Cached: 400}
	for _, host := range []string{"127.0.0.1", "host.docker.internal", "localhost", "192.168.1.20"} {
		cost, err := p.Cost(host, modelE, usage)
		if err != nil || cost != 0 {
			t.Errorf("Cost(%s, %s) = %v, %v; want 0", host, modelE, cost, err)
		}
	}
	if _, err := p.Cost("api.example.com", modelE, usage); !errors.Is(err, ErrNoPrice) {
		t.Errorf("an unlisted cloud endpoint: err = %v, want ErrNoPrice", err)
	}
}

const pricesFixture = `schema_version: 1
currency: USD
prices:
  - endpoint_host: API.Example.com
    model: big
    prompt_per_1k: 0.5
    completion_per_1k: 1.5
    cached_prompt_per_1k: 0.05
  - endpoint_host: api.example.com
    model: small
    prompt_per_1k: 0.1
    completion_per_1k: 0.2
  - endpoint_host: 2001:db8::1
    model: small
    prompt_per_1k: 1
    completion_per_1k: 1
`

func TestCost(t *testing.T) {
	p, err := ParsePrices([]byte(pricesFixture))
	if err != nil {
		t.Fatalf("ParsePrices: %v", err)
	}
	for _, tc := range []struct {
		name        string
		host, model string
		tokens      Tokens
		want        float64
	}{
		{"prompt and completion", "api.example.com", "small", Tokens{Prompt: 2000, Completion: 500}, 0.2 + 0.1},
		// 1000 prompt tokens of which 400 cached: 600*0.5 + 400*0.05 + 1000*1.5, per 1000.
		{"cached at its own price", "api.example.com", "big", Tokens{Prompt: 1000, Completion: 1000, Cached: 400}, 0.3 + 0.02 + 1.5},
		{"cached without a cached price costs as prompt", "api.example.com", "small", Tokens{Prompt: 1000, Cached: 1000}, 0.1},
		{"cached above prompt is clamped", "api.example.com", "big", Tokens{Prompt: 100, Cached: 900}, 0.005},
		{"host is case and dot insensitive", "API.EXAMPLE.COM.", "small", Tokens{Completion: 1000}, 0.2},
		{"ipv6 host", "2001:db8::1", "small", Tokens{Prompt: 1000}, 1},
		{"no tokens", "api.example.com", "big", Tokens{}, 0},
	} {
		got, err := p.Cost(tc.host, tc.model, tc.tokens)
		if err != nil {
			t.Errorf("%s: %v", tc.name, err)
			continue
		}
		if math.Abs(got-tc.want) > 1e-12 {
			t.Errorf("%s: Cost = %v, want %v", tc.name, got, tc.want)
		}
	}

	if _, err := p.Cost("api.example.com", "unknown", Tokens{Prompt: 1}); !errors.Is(err, ErrNoPrice) {
		t.Errorf("unknown model: err = %v, want ErrNoPrice", err)
	}
	if _, err := p.Cost("other.example.com", "small", Tokens{Prompt: 1}); !errors.Is(err, ErrNoPrice) {
		t.Errorf("unknown host: err = %v, want ErrNoPrice", err)
	}

	var empty *Prices
	if cost, err := empty.Cost("127.0.0.1", modelE, Tokens{Prompt: 10}); err != nil || cost != 0 {
		t.Errorf("nil table, local host: %v, %v; want 0", cost, err)
	}
	if _, err := empty.Cost("api.example.com", "small", Tokens{Prompt: 10}); !errors.Is(err, ErrNoPrice) {
		t.Errorf("nil table, cloud host: err = %v, want ErrNoPrice", err)
	}
}

func TestParsePricesRefusesBadTables(t *testing.T) {
	head := "schema_version: 1\ncurrency: USD\nprices:\n"
	entry := func(host, extra string) string {
		return "  - endpoint_host: " + host + "\n    model: m\n    prompt_per_1k: 0\n    completion_per_1k: 0\n" + extra
	}
	for name, tc := range map[string]struct{ doc, want string }{
		"empty file":         {"", "empty"},
		"no schema version":  {"currency: USD\nprices: []\n", "schema_version"},
		"future schema":      {"schema_version: 2\ncurrency: USD\nprices: []\n", "schema_version"},
		"other currency":     {"schema_version: 1\ncurrency: EUR\nprices: []\n", "currency"},
		"unknown key":        {head + entry("api.example.com", "    discount: 0.5\n"), "discount"},
		"scheme in the host": {head + entry("https://api.example.com", ""), "endpoint_host"},
		"port in the host":   {head + entry("api.example.com:443", ""), "endpoint_host"},
		"empty host":         {head + entry(`""`, ""), "endpoint_host"},
		"empty model":        {head + "  - endpoint_host: api.example.com\n    model: \"\"\n    prompt_per_1k: 0\n    completion_per_1k: 0\n", "model"},
		"negative price":     {head + "  - endpoint_host: api.example.com\n    model: m\n    prompt_per_1k: -1\n    completion_per_1k: 0\n", "prompt_per_1k"},
		"negative cached":    {head + entry("api.example.com", "    cached_prompt_per_1k: -0.1\n"), "cached_prompt_per_1k"},
		"infinite price":     {head + "  - endpoint_host: api.example.com\n    model: m\n    prompt_per_1k: 0\n    completion_per_1k: .inf\n", "completion_per_1k"},
		"priced twice":       {head + entry("api.example.com", "") + entry("API.example.com", ""), "twice"},
		"paid local host":    {head + "  - endpoint_host: 127.0.0.1\n    model: m\n    prompt_per_1k: 0.1\n    completion_per_1k: 0\n", "local"},
		"paid cached local":  {head + entry("host.docker.internal", "    cached_prompt_per_1k: 0.1\n"), "local"},
		// Iteration 2 (review Mi-5, N-2, N-4).
		"paid completion local":   {head + "  - endpoint_host: localhost\n    model: m\n    prompt_per_1k: 0\n    completion_per_1k: 0.1\n", "local"},
		"paid mapped local":       {head + "  - endpoint_host: \"::ffff:127.0.0.1\"\n    model: m\n    prompt_per_1k: 0\n    completion_per_1k: 0.1\n", "local"},
		"ipv6 priced twice":       {head + entry(`"2001:db8::1"`, "") + entry(`"2001:DB8:0:0::1"`, ""), "twice"},
		"two documents":           {head + entry("api.example.com", "") + "---\n" + head + entry("api.other.com", ""), "one YAML document"},
		"garbage second document": {head + entry("api.example.com", "") + "---\n: : :\n", "one YAML document"},
	} {
		_, err := ParsePrices([]byte(tc.doc))
		if !errors.Is(err, ErrInvalidPrices) {
			t.Errorf("%s: err = %v, want ErrInvalidPrices", name, err)
			continue
		}
		if !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: error %q does not mention %q", name, err, tc.want)
		}
	}
}

func TestLoadPricesReportsAMissingFile(t *testing.T) {
	_, err := LoadPrices(filepath.Join(t.TempDir(), "absent.yaml"))
	if err == nil || !strings.Contains(err.Error(), "price table") {
		t.Fatalf("missing file: err = %v", err)
	}
}

// Iteration 2 (review N-2, N-5): the host is compared in one canonical form,
// and Cost takes the authority of a URL as well as a bare host.
func TestCostNormalisesTheHost(t *testing.T) {
	p, err := ParsePrices([]byte("schema_version: 1\ncurrency: USD\nprices:\n" +
		"  - endpoint_host: \"2001:DB8:0:0::1\"\n    model: m\n    prompt_per_1k: 1\n    completion_per_1k: 0\n" +
		"  - endpoint_host: api.example.com\n    model: m\n    prompt_per_1k: 2\n    completion_per_1k: 0\n"))
	if err != nil {
		t.Fatalf("ParsePrices: %v", err)
	}
	one := Tokens{Prompt: 1000}
	for host, want := range map[string]float64{
		"2001:db8::1":         1,
		"[2001:db8::1]":       1,
		"[2001:DB8::1]:8443":  1,
		"api.example.com":     2,
		"API.example.com.":    2,
		"api.example.com:443": 2,
		"127.0.0.1:8888":      0,
		"[::1]":               0,
		"[::1]:8080":          0,
		"::1":                 0,
		"localhost:1234":      0,
		"[::ffff:10.0.0.1]:1": 0,
	} {
		got, err := p.Cost(host, "m", one)
		if err != nil || got != want {
			t.Errorf("Cost(%q) = %v, %v; want %v", host, got, err, want)
		}
	}

	u := "http://[2001:DB8:0::1]:8080/v1"
	host, err := EndpointHost(u)
	if err != nil {
		t.Fatalf("EndpointHost(%s): %v", u, err)
	}
	if got, err := p.Cost(host, "m", one); err != nil || got != 1 {
		t.Errorf("Cost(EndpointHost(%s)) = %v, %v; want 1", u, got, err)
	}
}
