package llm

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"net/netip"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// PricesSchemaVersion is the layout of config/llm-prices.yaml this package reads.
const PricesSchemaVersion = 1

// ErrNoPrice is a non-local (endpoint_host, model) the table does not price.
// It is an error rather than a zero: a call whose cost is unknown must not
// look free to the cloud budget.
var ErrNoPrice = errors.New("llm: no price for this endpoint and model")

// ErrInvalidPrices is what every complaint about the price table matches.
var ErrInvalidPrices = errors.New("llm: invalid price table")

// PricesDocument is config/llm-prices.yaml as it is written. Every key is
// required unless its comment says otherwise; an unknown key is refused, so a
// typo cannot quietly make a model free.
type PricesDocument struct {
	SchemaVersion int `yaml:"schema_version"`
	// Currency is USD: the cost lands in llm.output.cost_usd.
	Currency string       `yaml:"currency"`
	Prices   []PriceEntry `yaml:"prices"`
}

// PriceEntry is the price of one model at one endpoint, per 1000 tokens.
type PriceEntry struct {
	EndpointHost    string  `yaml:"endpoint_host"`
	Model           string  `yaml:"model"`
	PromptPer1K     float64 `yaml:"prompt_per_1k"`
	CompletionPer1K float64 `yaml:"completion_per_1k"`
	// CachedPromptPer1K is optional: the price of the prompt tokens served from
	// the cache of the provider. Absent, they cost as prompt tokens.
	CachedPromptPer1K *float64 `yaml:"cached_prompt_per_1k"`
}

type priceKey struct{ host, model string }

// Prices is a loaded price table. A nil *Prices is an empty table: local
// endpoints still cost 0, any other one has no price.
type Prices struct {
	entries map[priceKey]PriceEntry
}

// LoadPrices reads the price table at path.
func LoadPrices(path string) (*Prices, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("llm: read price table: %w", err)
	}
	return ParsePrices(data)
}

// ParsePrices parses a price table.
func ParsePrices(data []byte) (*Prices, error) {
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	var doc PricesDocument
	if err := dec.Decode(&doc); err != nil {
		if errors.Is(err, io.EOF) {
			return nil, badPrices("the file is empty")
		}
		return nil, badPrices("%v", err)
	}
	// A second document after --- would be ignored by a single Decode, and a
	// table the author extended below a separator would silently lose its
	// entries.
	var extra yaml.Node
	if err := dec.Decode(&extra); !errors.Is(err, io.EOF) {
		return nil, badPrices("one YAML document expected, found more")
	}
	if doc.SchemaVersion != PricesSchemaVersion {
		return nil, badPrices("schema_version is %d, this build reads %d", doc.SchemaVersion, PricesSchemaVersion)
	}
	if doc.Currency != "USD" {
		return nil, badPrices("currency is %q, the cost is recorded in USD", doc.Currency)
	}

	p := &Prices{entries: make(map[priceKey]PriceEntry, len(doc.Prices))}
	for i, e := range doc.Prices {
		at := fmt.Sprintf("prices[%d]", i)
		host := canonicalHost(e.EndpointHost)
		if !validHost(host) {
			return nil, badPrices("%s.endpoint_host %q is not a host: no scheme, port or path", at, e.EndpointHost)
		}
		if strings.TrimSpace(e.Model) == "" {
			return nil, badPrices("%s.model is empty", at)
		}
		if !validPrice(e.PromptPer1K) {
			return nil, badPrices("%s.prompt_per_1k must be a non-negative number, got %v", at, e.PromptPer1K)
		}
		if !validPrice(e.CompletionPer1K) {
			return nil, badPrices("%s.completion_per_1k must be a non-negative number, got %v", at, e.CompletionPer1K)
		}
		if e.CachedPromptPer1K != nil && !validPrice(*e.CachedPromptPer1K) {
			return nil, badPrices("%s.cached_prompt_per_1k must be a non-negative number, got %v", at, *e.CachedPromptPer1K)
		}
		if isLocalHost(host) && (e.PromptPer1K != 0 || e.CompletionPer1K != 0 ||
			(e.CachedPromptPer1K != nil && *e.CachedPromptPer1K != 0)) {
			return nil, badPrices("%s: %s is a local endpoint and local endpoints cost 0 (ADR-005 add. 2 p. 3)", at, host)
		}
		key := priceKey{host: host, model: e.Model}
		if _, dup := p.entries[key]; dup {
			return nil, badPrices("%s: (%s, %s) is priced twice", at, host, e.Model)
		}
		e.EndpointHost = host
		p.entries[key] = e
	}
	return p, nil
}

// Cost is the cost in USD of one call with usage t at (host, model). A local
// host costs 0 whatever the table says. Cached tokens are part of the prompt
// tokens and are billed at the cached price when the entry has one.
//
// host is meant to be the result of EndpointHost. The authority of a URL is
// accepted as well — "127.0.0.1:8888", "[::1]", "[::1]:8080" — because a
// local call reported as having no price is an error nobody would look for.
func (p *Prices) Cost(host, model string, t Tokens) (float64, error) {
	host = canonicalHost(hostOfAuthority(host))
	if isLocalHost(host) {
		return 0, nil
	}
	var (
		e  PriceEntry
		ok bool
	)
	if p != nil {
		e, ok = p.entries[priceKey{host: host, model: model}]
	}
	if !ok {
		return 0, fmt.Errorf("%w: (%s, %s)", ErrNoPrice, host, model)
	}
	cached := min(max(t.Cached, 0), max(t.Prompt, 0))
	uncached := max(t.Prompt, 0) - cached
	cachedPrice := e.PromptPer1K
	if e.CachedPromptPer1K != nil {
		cachedPrice = *e.CachedPromptPer1K
	}
	cost := (float64(uncached)*e.PromptPer1K +
		float64(cached)*cachedPrice +
		float64(max(t.Completion, 0))*e.CompletionPer1K) / 1000
	return cost, nil
}

// hostOfAuthority drops a port and IPv6 brackets. A bare IPv6 address has
// colons of its own and is returned as it is.
func hostOfAuthority(s string) string {
	s = strings.TrimSpace(s)
	if h, _, err := net.SplitHostPort(s); err == nil {
		return h
	}
	if strings.HasPrefix(s, "[") && strings.HasSuffix(s, "]") {
		return s[1 : len(s)-1]
	}
	return s
}

func validPrice(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) && v >= 0 }

// validHost accepts a DNS name or an IP address and nothing that carries a
// scheme, a port or a path — the key must match EndpointHost of a URL.
func validHost(host string) bool {
	if host == "" {
		return false
	}
	if _, err := netip.ParseAddr(host); err == nil {
		return true
	}
	return !strings.ContainsAny(host, ":/ ?#@")
}

func badPrices(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrInvalidPrices, fmt.Sprintf(format, args...))
}
