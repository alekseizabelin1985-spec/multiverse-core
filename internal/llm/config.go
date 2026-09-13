package llm

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net/netip"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	"multiverse-core.io/shared/env"
)

// The values of MV_LLM_PROVIDER (ADR-005 add. 2 p. 2). The manifest in
// shared/env holds the same set; a test keeps the two equal.
const (
	ProviderOpenAICompat = "openai_compat"
	ProviderOllama       = "ollama"
	ProviderAnthropic    = "anthropic"
	ProviderRecorded     = "recorded"
	ProviderFake         = "fake"
)

// retiredProviders are names that used to select a provider and now are an
// address and a key of openai_compat. They get their own sentence instead of
// "unknown value": an operator who still writes them is one edit away from a
// working .env and should be told which edit.
var retiredProviders = map[string]bool{"openai": true, "deepseek": true}

// ErrConfig is what every complaint about the configuration of the block
// matches; the detail is in the ConfigError it wraps.
var ErrConfig = errors.New("llm: invalid configuration")

// ErrCloudDisabled is a non-local endpoint without MV_LLM_CLOUD_ENABLED=true
// (ADR-005 add. 2 p. 3). It also matches ErrConfig.
var ErrCloudDisabled = errors.New("llm: cloud endpoint without MV_LLM_CLOUD_ENABLED=true")

// ConfigError is one rejected variable. It never carries the value of a
// secret or a whole URL: a URL may hold credentials in its user part, so only
// its host is ever named.
type ConfigError struct {
	Var    string
	Reason string
	err    error
}

func (e *ConfigError) Error() string {
	return fmt.Sprintf("llm: invalid configuration: %s: %s", e.Var, e.Reason)
}

// Is makes every ConfigError match ErrConfig, and the gate refusal match
// ErrCloudDisabled as well.
func (e *ConfigError) Is(target error) bool {
	return target == ErrConfig || (e.err != nil && target == e.err)
}

func badVar(name, format string, args ...any) error {
	return &ConfigError{Var: name, Reason: fmt.Sprintf(format, args...)}
}

// Secret is a value that must not reach a log, an error or an event (ADR-009
// p. 4, SEC-21). Every way fmt, slog, encoding/json and yaml print it yields a
// placeholder; Reveal is the one place the value comes out, for the header of
// a request. The zero Secret is empty.
//
// The value sits behind a pointer to a string, and that is the point of the
// type, not an accident. Methods are not enough on their own: when fmt meets a
// verb that does not fit (%p, %w, %t on a Config or on a pointer to it) it
// reports %!p(llm.Config=…) and prints the whole value by reflection with
// every method switched off — String and Format included. A string field would
// be printed there in clear. A pointer to a string is printed by reflection as
// an address, and fmt never follows it, so even that path shows no key.
type Secret struct{ v *string }

// NewSecret wraps value; an empty value gives the zero, empty Secret.
func NewSecret(value string) Secret {
	if value == "" {
		return Secret{}
	}
	return Secret{v: &value}
}

const redacted = "[redacted]"

// IsSet reports whether the secret carries a value.
func (s Secret) IsSet() bool { return s.v != nil }

// String hides the value; an empty secret prints empty so a log still shows
// whether a key is configured.
func (s Secret) String() string {
	if !s.IsSet() {
		return ""
	}
	return redacted
}

// Format hides the value from fmt under every verb a Formatter is asked for,
// the placeholder instead of the %!d(…) report.
func (s Secret) Format(f fmt.State, _ rune) { _, _ = io.WriteString(f, s.String()) }

// LogValue hides the value from slog.
func (s Secret) LogValue() slog.Value { return slog.StringValue(s.String()) }

// MarshalText hides the value from every encoder that honours
// encoding.TextMarshaler: encoding/json, gopkg.in/yaml.v3, encoding/xml.
func (s Secret) MarshalText() ([]byte, error) { return []byte(s.String()), nil }

// MarshalYAML hides the value from yaml.v3 even if it stops consulting
// TextMarshaler for a string kind.
func (s Secret) MarshalYAML() (any, error) { return s.String(), nil }

// Reveal returns the value itself.
func (s Secret) Reveal() string {
	if !s.IsSet() {
		return ""
	}
	return *s.v
}

// Cloud is the operator's permission to leave the machine (BR-15).
type Cloud struct {
	Enabled              bool
	AllowExternalPlayers bool
	BudgetUSDPerDay      float64
}

// Timeouts bound one attempt of each phase.
type Timeouts struct {
	Narrative time.Duration
	Tick      time.Duration
	Decision  time.Duration
	// Degraded replaces the timeout of the phase while the provider is
	// unavailable (КД EPIC-003 §9.5).
	Degraded time.Duration
}

// For returns the timeout of one attempt of phase p.
func (t Timeouts) For(p Phase) (time.Duration, error) {
	switch p {
	case PhaseNarrative:
		return t.Narrative, nil
	case PhaseTick:
		return t.Tick, nil
	case PhaseDecision:
		return t.Decision, nil
	}
	return 0, fmt.Errorf("llm: no timeout is configured for phase %q", p)
}

// Config is the configuration of the LLM block, read from the manifest of
// shared/env. The model is not here: it comes from the blueprint of a phase
// (Call.Model), and a blueprint never names a provider (SEC-21).
type Config struct {
	Provider string
	// URL is the base address of MV_LLM_URL with a trailing /v1 trimmed; the
	// path under it is appended by the provider (C-15, T-404). Empty when the
	// provider does not use it.
	URL    string
	APIKey Secret
	// OllamaURL is read only with MV_LLM_PROVIDER=ollama.
	OllamaURL    string
	NumCtx       int
	StorePrompts bool
	Cloud        Cloud
	Timeouts     Timeouts
	// PricesPath is the file of the price table (MV_LLM_PRICES).
	PricesPath string
}

// LoadConfig reads the configuration from src; a nil src is the process
// environment. Every variable is read through its declaration in shared/env,
// so a name, a default and a kind exist in one place only.
func LoadConfig(src env.Source) (Config, error) {
	var errs []error
	collect := func(err error) {
		if err != nil {
			errs = append(errs, err)
		}
	}

	cfg := Config{Provider: env.LLMProvider.StringFrom(src)}
	collect(CheckProvider(cfg.Provider))

	if raw := env.LLMURL.StringFrom(src); raw != "" {
		u, err := normalizeURL(env.LLMURL.Name(), raw)
		collect(err)
		cfg.URL = u
	} else if usesLLMURL(cfg.Provider) {
		collect(badVar(env.LLMURL.Name(), "required by provider %s and empty", cfg.Provider))
	}
	cfg.APIKey = NewSecret(env.LLMAPIKey.StringFrom(src))

	if cfg.Provider == ProviderOllama {
		raw := env.OllamaURL.StringFrom(src)
		if raw == "" {
			collect(badVar(env.OllamaURL.Name(), "required by provider %s and empty", ProviderOllama))
		} else {
			u, err := normalizeURL(env.OllamaURL.Name(), raw)
			collect(err)
			cfg.OllamaURL = u
		}
	}

	var err error
	cfg.NumCtx, err = positiveInt(env.LLMNumCtx, src)
	collect(err)
	cfg.StorePrompts, err = env.LLMStorePrompts.BoolFrom(src)
	collect(err)
	cfg.Cloud.Enabled, err = env.LLMCloudEnabled.BoolFrom(src)
	collect(err)
	cfg.Cloud.AllowExternalPlayers, err = env.LLMCloudAllowExternalPlayers.BoolFrom(src)
	collect(err)
	cfg.Cloud.BudgetUSDPerDay, err = nonNegativeAmount(env.LLMCloudBudgetUSDPerDay, src)
	collect(err)

	cfg.Timeouts.Narrative, err = positiveDuration(env.LLMTimeoutNarrative, src)
	collect(err)
	cfg.Timeouts.Tick, err = positiveDuration(env.LLMTimeoutTick, src)
	collect(err)
	cfg.Timeouts.Decision, err = positiveDuration(env.LLMTimeoutDecision, src)
	collect(err)
	cfg.Timeouts.Degraded, err = positiveDuration(env.LLMTimeoutDegraded, src)
	collect(err)

	cfg.PricesPath = env.LLMPrices.StringFrom(src)
	if cfg.PricesPath == "" {
		collect(badVar(env.LLMPrices.Name(), "empty: the cost of a call cannot be accounted without the price table"))
	}

	if len(errs) > 0 {
		return Config{}, errors.Join(errs...)
	}
	return cfg, nil
}

// CheckProvider says whether name is a value of MV_LLM_PROVIDER. A retired
// name (openai, deepseek) gets the sentence that names its replacement.
func CheckProvider(name string) error {
	if retiredProviders[name] {
		return badVar(env.LLMProvider.Name(),
			"%q is no longer a provider name (ADR-005 add. 2): use %s with the address in %s and the key in %s",
			name, ProviderOpenAICompat, env.LLMURL.Name(), env.LLMAPIKey.Name())
	}
	if enum := env.LLMProvider.Enum(); !slices.Contains(enum, name) {
		return badVar(env.LLMProvider.Name(), "%q is not one of %s", name, strings.Join(enum, ", "))
	}
	return nil
}

// usesLLMURL says which providers talk to MV_LLM_URL. ollama has its own
// address, recorded and fake do not talk to the network at all.
func usesLLMURL(provider string) bool {
	return provider == ProviderOpenAICompat || provider == ProviderAnthropic
}

// Endpoint is the address the configured provider talks to, empty for a
// provider that does not leave the process.
func (c Config) Endpoint() (name, address string) {
	switch {
	case usesLLMURL(c.Provider):
		return env.LLMURL.Name(), c.URL
	case c.Provider == ProviderOllama:
		return env.OllamaURL.Name(), c.OllamaURL
	}
	return "", ""
}

// CheckCloudGate refuses a non-local endpoint unless the operator enabled the
// cloud (ADR-005 add. 2 p. 3). The gate is the address, not the provider name:
// openai_compat pointed at a vendor is the cloud, openai_compat pointed at
// llama-server on loopback is not. providers.New calls it before any factory,
// so no provider starts past it.
//
// The gate does not trust that the Config came from LoadConfig: a provider
// that needs an address and has none is refused, and the address passes the
// same checks as in LoadConfig (credentials, query, fragment, port). A factory
// therefore never sees an address LoadConfig would have refused, whoever
// assembled the Config.
func (c Config) CheckCloudGate() error {
	name, address := c.Endpoint()
	if name == "" {
		return nil
	}
	if address == "" {
		return badVar(name, "required by provider %s and empty", c.Provider)
	}
	u, err := parseEndpoint(name, address)
	if err != nil {
		return err
	}
	host := canonicalHost(u.Hostname())
	if isLocalHost(host) || c.Cloud.Enabled {
		return nil
	}
	return &ConfigError{
		Var: name,
		Reason: fmt.Sprintf("host %s is not a local address (loopback, host.docker.internal, RFC 1918); "+
			"a cloud endpoint needs %s=true", host, env.LLMCloudEnabled.Name()),
		err: ErrCloudDisabled,
	}
}

// LogValue is what slog prints for a Config: the key only as "set or not".
func (c Config) LogValue() slog.Value {
	_, address := c.Endpoint()
	host, _ := EndpointHost(address)
	return slog.GroupValue(
		slog.String("provider", c.Provider),
		slog.String("endpoint_host", host),
		slog.Bool("api_key_set", c.APIKey.IsSet()),
		slog.Bool("cloud_enabled", c.Cloud.Enabled),
	)
}

// IsLocalEndpoint reports whether the host of rawURL is local: loopback,
// host.docker.internal or a private IPv4 range of RFC 1918. Everything else —
// a public address, a DNS name, an IPv6 unique local address — is the cloud:
// the gate fails closed, and a name is not resolved, because a resolution
// would make the answer depend on the DNS of the moment.
func IsLocalEndpoint(rawURL string) (bool, error) {
	host, err := EndpointHost(rawURL)
	if err != nil {
		return false, err
	}
	return isLocalHost(host), nil
}

// EndpointHost returns the host of an absolute http(s) URL in its canonical
// form — the endpoint_host of the price table: lower case, without the port and
// a trailing dot, an IP address in the form netip prints it (so that
// [2001:DB8:0::1] and 2001:db8::1 are one key).
func EndpointHost(rawURL string) (string, error) {
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" {
		return "", errors.New("not an absolute http(s) URL")
	}
	return canonicalHost(u.Hostname()), nil
}

// canonicalHost is the one spelling of a host every comparison of this package
// uses: the gate, the price table and Cost.
func canonicalHost(host string) string {
	host = strings.TrimSuffix(strings.ToLower(strings.TrimSpace(host)), ".")
	if addr, err := netip.ParseAddr(host); err == nil {
		return addr.Unmap().String()
	}
	return host
}

var rfc1918 = []netip.Prefix{
	netip.MustParsePrefix("10.0.0.0/8"),
	netip.MustParsePrefix("172.16.0.0/12"),
	netip.MustParsePrefix("192.168.0.0/16"),
}

// isLocalHost takes a host in the form of canonicalHost; every caller passes
// one, so an IPv4-mapped address is already unmapped here.
func isLocalHost(host string) bool {
	if host == "localhost" || host == "host.docker.internal" {
		return true
	}
	addr, err := netip.ParseAddr(host)
	if err != nil {
		return false
	}
	if addr.IsLoopback() {
		return true
	}
	for _, p := range rfc1918 {
		if p.Contains(addr) {
			return true
		}
	}
	return false
}

// normalizeURL checks an endpoint and reduces it to its base address: a
// trailing slash and a trailing /v1 are trimmed (C-15, T-404). A user part is
// refused rather than kept: credentials belong in MV_LLM_API_KEY, which is a
// secret, while a URL ends up in logs and errors.
func normalizeURL(name, raw string) (string, error) {
	u, err := parseEndpoint(name, raw)
	if err != nil {
		return "", err
	}
	u.Path = strings.TrimSuffix(strings.TrimRight(u.Path, "/"), "/v1")
	u.Path = strings.TrimRight(u.Path, "/")
	u.RawPath = ""
	return u.String(), nil
}

// parseEndpoint holds every rule an endpoint must pass, for LoadConfig and for
// the gate alike. Errors name the host at most, never the value.
//
//   - an absolute http(s) URL;
//   - no user part: credentials belong in MV_LLM_API_KEY;
//   - no query and no fragment, not even an empty "?" or "#": the provider
//     appends a path to the base address, and a "?" left in it would turn that
//     path into a query (scripts/lib/llm-endpoint.sh refuses both too);
//   - a port, when written, in 1-65535;
//   - a loopback or host.docker.internal address carries its port: without one
//     the scheme default (80) is a port nobody chose, the same rule as
//     llm-endpoint.sh (review Mi-12 of T-404).
func parseEndpoint(name, raw string) (*url.URL, error) {
	raw = strings.TrimSpace(raw)
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" {
		return nil, badVar(name, "not an absolute http(s) URL")
	}
	host := u.Hostname()
	if u.User != nil {
		return nil, badVar(name, "carries credentials in the URL; put the key in %s", env.LLMAPIKey.Name())
	}
	if strings.ContainsAny(raw, "?#") {
		return nil, badVar(name, "a base address has no query or fragment (host %s)", host)
	}
	port := u.Port()
	if port == "" && strings.HasSuffix(u.Host, ":") {
		return nil, badVar(name, "the port after %s is empty", host)
	}
	if port != "" {
		if n, err := strconv.Atoi(port); err != nil || n < 1 || n > 65535 {
			return nil, badVar(name, "port %s of host %s is outside 1-65535", port, host)
		}
	} else if needsExplicitPort(host) {
		return nil, badVar(name, "host %s has no port; a local runtime would be reached on %s, the default "+
			"of the scheme — write the port you mean", host, defaultPort(u.Scheme))
	}
	return u, nil
}

// needsExplicitPort is the set of llm_host_is_local in llm-endpoint.sh that a
// URL can name: the addresses a runtime started on this machine answers on.
// A LAN or a vendor address keeps the scheme default.
func needsExplicitPort(host string) bool {
	host = canonicalHost(host)
	if host == "localhost" || host == "host.docker.internal" {
		return true
	}
	addr, err := netip.ParseAddr(host)
	return err == nil && (addr.IsLoopback() || addr.IsUnspecified())
}

func defaultPort(scheme string) string {
	if scheme == "https" {
		return "443"
	}
	return "80"
}

func positiveInt(v env.Var, src env.Source) (int, error) {
	n, err := v.IntFrom(src)
	if err != nil {
		return 0, err
	}
	if n <= 0 {
		return 0, badVar(v.Name(), "must be positive, got %d", n)
	}
	return n, nil
}

func positiveDuration(v env.Var, src env.Source) (time.Duration, error) {
	d, err := v.DurationFrom(src)
	if err != nil {
		return 0, err
	}
	if d <= 0 {
		return 0, badVar(v.Name(), "must be positive, got %s", d)
	}
	return d, nil
}

// nonNegativeAmount parses an amount of money. The manifest has no kind for a
// fraction, so the parse is here.
func nonNegativeAmount(v env.Var, src env.Source) (float64, error) {
	raw := v.StringFrom(src)
	f, err := strconv.ParseFloat(raw, 64)
	if err != nil || math.IsNaN(f) || math.IsInf(f, 0) || f < 0 {
		return 0, badVar(v.Name(), "must be a non-negative amount, got %q", raw)
	}
	return f, nil
}
