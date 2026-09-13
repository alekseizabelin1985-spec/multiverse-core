package llm

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net/netip"
	"net/url"
	"regexp"
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

	if raw := addressFrom(env.LLMURL, src); raw != "" {
		u, err := normalizeURL(env.LLMURL.Name(), raw)
		collect(err)
		cfg.URL = u
	} else if usesLLMURL(cfg.Provider) {
		collect(badVar(env.LLMURL.Name(), "required by provider %s and empty", cfg.Provider))
	}
	cfg.APIKey = NewSecret(env.LLMAPIKey.StringFrom(src))

	if cfg.Provider == ProviderOllama {
		raw := addressFrom(env.OllamaURL, src)
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

// asciiSpace is the whitespace an address loses at its edges: ASCII only, as
// llm_trim of the scripts. strings.TrimSpace strips U+00A0, U+2003 and U+3000
// as well, and would make local a value C-15 calls invalid.
const asciiSpace = " \t\n\v\f\r"

// addressFrom is the value of an address variable less the ASCII whitespace at
// its edges. shared/env hands every value out through strings.TrimSpace, so an
// address with a no-break space at an edge would reach parseEndpoint already
// clean and start the platform on a value compose-lint refuses; the address is
// therefore read from the source itself, under the name of its declaration.
func addressFrom(v env.Var, src env.Source) string {
	if src == nil {
		src = env.OS()
	}
	if raw, ok := src(v.Name()); ok {
		return strings.Trim(raw, asciiSpace)
	}
	return v.StringFrom(src)
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
	notLocal := " is not a local address (loopback, localhost, host.docker.internal, a one-word compose service, " +
		"RFC 1918, link-local, IPv6 ULA); a cloud endpoint needs " + env.LLMCloudEnabled.Name() + "=true"
	return &ConfigError{
		Var:    name,
		Reason: aboutTheHost(address, "the host"+notLocal, "host %s%s", host, notLocal),
		err:    ErrCloudDisabled,
	}
}

// LogValue is what slog prints for a Config: the key only as "set or not", and
// the host of the address under the rule of the refusals (aboutTheHost): an
// address with an @ anywhere may hold the head of a password where url.Parse
// sees a host (http://pw:2024/x@10.0.0.5:8080 has host pw), so its host is not
// printed (T-451 review #2, acceptance).
func (c Config) LogValue() slog.Value {
	_, address := c.Endpoint()
	host, _ := EndpointHost(address)
	return slog.GroupValue(
		slog.String("provider", c.Provider),
		slog.String("endpoint_host", aboutTheHost(address, "withheld", "%s", host)),
		slog.Bool("api_key_set", c.APIKey.IsSet()),
		slog.Bool("cloud_enabled", c.Cloud.Enabled),
	)
}

// IsLocalEndpoint answers the rule "local address" of C-15 v1.3 for rawURL.
// The cases of the rule are testdata/llm/local-endpoints.tsv, and where this
// comment and the table disagree the table is right:
//
//   - true (local) — loopback 127.0.0.0/8 and ::1; localhost and *.localhost;
//     host.docker.internal; a one-word name, a service of compose; RFC 1918;
//     link-local 169.254.0.0/16 and fe80::/10; IPv6 ULA fc00::/7; the
//     IPv4-mapped form of each;
//   - an error matching ErrConfig (invalid) — the any-address 0.0.0.0 or ::, a
//     local host without a port, and every value parseEndpoint refuses;
//   - false (cloud) — everything else.
//
// A name is never resolved: the class must not depend on the DNS of the moment.
func IsLocalEndpoint(rawURL string) (bool, error) {
	u, err := parseEndpoint("URL", rawURL)
	if err != nil {
		return false, err
	}
	return isLocalHost(canonicalHost(u.Hostname())), nil
}

// EndpointHost returns the host of an absolute http(s) URL in its canonical
// form — the endpoint_host of the price table: see canonicalHost.
func EndpointHost(rawURL string) (string, error) {
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" {
		return "", errors.New("not an absolute http(s) URL")
	}
	return canonicalHost(u.Hostname()), nil
}

// canonicalHost is the one spelling of a host every comparison of this package
// uses: the gate, the price table and Cost. Lower case; an IP address in the
// form netip prints it, unmapped (so that [2001:DB8:0::1] and 2001:db8::1 are
// one key, and ::ffff:10.0.0.1 is 10.0.0.1).
//
// The trailing dot of a name is the root of DNS, and it is kept wherever
// dropping it would change the class (T-450, C-15 v1.3):
//   - a dotted quad with it (127.0.0.1.) is not an address to netip, and Go's
//     client hands it to DNS as a name — dropped, it would become loopback;
//   - a one-word name with it (ollama.) is an absolute DNS name that skips the
//     search list, not a service of compose — dropped, it would become one.
//
// It is dropped from localhost. (RFC 6761) and from a name with a dot inside:
// such a name is local only when it is *.localhost or host.docker.internal,
// which the rule reads without the dot, and the cloud with or without it, so
// api.example.com. stays one price key with api.example.com.
//
// One dot is the root; a second one is an empty label, and such a name is kept
// whole. So canonicalHost of its own result is that result: Cost canonicalizes
// the host EndpointHost has already canonicalized, and localhost.. dropped to
// localhost. there would become localhost, local and free, on the second pass
// (T-451 review #1 Mi-2).
func canonicalHost(host string) string {
	host = strings.ToLower(strings.TrimSpace(host))
	if addr, err := netip.ParseAddr(host); err == nil {
		return addr.Unmap().String()
	}
	bare, dotted := strings.CutSuffix(host, ".")
	if !dotted || strings.HasSuffix(bare, ".") {
		return host
	}
	if _, err := netip.ParseAddr(bare); err == nil {
		return host
	}
	if bare == "localhost" || strings.Contains(bare, ".") {
		return bare
	}
	return host
}

// hostClass is the answer of the rule for a host, before its port is looked at.
type hostClass int

const (
	hostCloud hostClass = iota
	hostLocal
	// hostAnyAddress is 0.0.0.0 or :: — where a server listens, not an address
	// a client can call: a configuration error, not the cloud.
	hostAnyAddress
)

var (
	// subLocalhost is *.localhost: every label [a-z0-9_-], none empty.
	subLocalhost = regexp.MustCompile(`^([a-z0-9_-]+\.)+localhost$`)
	// serviceName is a one-word name that starts with a letter: 2130706433 and
	// 0x08080808 are numeric spellings of an address a resolver with the
	// semantics of inet_aton reads as 8.8.8.8, not names of a service.
	serviceName = regexp.MustCompile(`^[a-z][a-z0-9_-]*$`)
)

// classifyHost takes a host in the form of canonicalHost; every caller passes
// one, so an IPv4-mapped address is already unmapped here.
func classifyHost(host string) hostClass {
	if addr, err := netip.ParseAddr(host); err == nil {
		switch {
		case addr.IsUnspecified():
			return hostAnyAddress
		case addr.IsLoopback(), addr.IsPrivate(), addr.IsLinkLocalUnicast():
			return hostLocal
		}
		return hostCloud
	}
	if host == "localhost" || host == "host.docker.internal" ||
		subLocalhost.MatchString(host) || serviceName.MatchString(host) {
		return hostLocal
	}
	return hostCloud
}

func isLocalHost(host string) bool { return classifyHost(host) == hostLocal }

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

// parseEndpoint holds every rule an endpoint must pass, for LoadConfig, for the
// gate and for IsLocalEndpoint alike: a value it refuses is the class invalid of
// C-15, a configuration error and never ErrCloudDisabled.
//
//   - printable ASCII once the ASCII whitespace at the edges is gone, as the
//     scripts check it: a no-break space at an edge, a full-width letter, a
//     soft hyphen or an ideographic full stop is invalid, not a spelling of
//     localhost or of a vendor;
//   - an absolute http(s) URL that Go's url.Parse accepts — so a host with a
//     space, \ ^ ` { | }, an unclosed or malformed IPv6 literal, brackets
//     around anything but an IPv6 address, or a non-numeric port is refused;
//   - no user part, not even an empty one: credentials belong in MV_LLM_API_KEY;
//   - no percent sign in the authority: an escape or the zone of an IPv6
//     literal. It is looked for in the value, not in the parsed host: url.Parse
//     unescapes %C3%A9 there, and ol%C3%A9ma would reach the rule as olèma;
//   - no query and no fragment, not even an empty "?" or "#": the provider
//     appends a path to the base address, and a "?" left in it would turn that
//     path into a query;
//   - a port, when written, in 1-65535 (leading zeros are not significant);
//   - not the any-address 0.0.0.0 or ::;
//   - a local host carries its port: without one the scheme default is a port
//     nobody chose.
//
// No sentence prints the value. The error of url.Parse quotes it whole, and a
// password is not always where a parser looks for one: in
// http://u:pw/x@127.0.0.1 the authority is u:pw, and url.Parse reports the
// password as a port. A sentence names the host and the port of a value
// url.Parse has accepted, and not even those when the value holds an @: see
// aboutTheHost.
func parseEndpoint(name, raw string) (*url.URL, error) {
	raw = strings.Trim(raw, asciiSpace)
	if !printableASCII(raw) {
		return nil, badVar(name, "carries a character outside printable ASCII (a no-break space, a full-width letter, "+
			"a soft hyphen); retype the address by hand — the value is not printed: it may hold a key")
	}
	if !strings.Contains(raw, "://") {
		return nil, badVar(name, "has no scheme or no // after it: an address is an absolute http(s) URL, "+
			"http://host:port")
	}
	u, err := url.Parse(raw)
	if err != nil {
		return nil, badVar(name, "is not an absolute http(s) URL the HTTP client accepts (a character a host cannot "+
			"hold, a malformed port or IPv6 literal); the value is not printed: it may hold a key")
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, badVar(name, "is not an absolute http(s) URL: only http and https are addresses of an endpoint")
	}
	if u.User != nil {
		return nil, badVar(name, "carries credentials in the URL; put the key in %s", env.LLMAPIKey.Name())
	}
	if strings.Contains(authority(raw), "%") {
		return nil, badVar(name, "carries a percent sign in the host (an escaped character or the zone of an IPv6 "+
			"literal); write the host as it is")
	}
	host := u.Hostname()
	if host == "" {
		return nil, badVar(name, "has no host")
	}
	if strings.ContainsAny(raw, "?#") {
		return nil, badVar(name, "%s", aboutTheHost(raw,
			"a base address has no query or fragment",
			"a base address has no query or fragment (host %s; the rest is not printed: it may hold a key)", host))
	}
	// Go refuses http://::1:8888 by default, and GODEBUG=urlstrictcolons=0 makes
	// it host ::1 and port 8888. The class of a value must not follow a switch
	// of the runtime.
	if !strings.HasPrefix(u.Host, "[") && strings.Contains(host, ":") {
		return nil, badVar(name, "has a bare IPv6 literal; bracket it: http://[::1]:8888")
	}
	port := u.Port()
	if port == "" && strings.HasSuffix(u.Host, ":") {
		return nil, badVar(name, "%s", aboutTheHost(raw, "the port is empty", "the port after %s is empty", host))
	}
	if port != "" {
		if n, err := strconv.Atoi(port); err != nil || n < 1 || n > 65535 {
			return nil, badVar(name, "%s", aboutTheHost(raw,
				"the port is outside 1-65535", "port %s of host %s is outside 1-65535", port, host))
		}
	}
	if classifyHost(canonicalHost(host)) == hostAnyAddress {
		const why = ": a server listens there, a client cannot call it — write 127.0.0.1 or the address of the " +
			"host, with the port"
		return nil, badVar(name, "%s", aboutTheHost(raw, "names the any-address"+why, "names %s, the any-address%s",
			host, why))
	}
	if port == "" && needsExplicitPort(host) {
		why := " has no port; a local address is written with the port its runtime listens on, and " +
			defaultPort(u.Scheme) + ", the default of the scheme, is a port nobody chose"
		return nil, badVar(name, "%s", aboutTheHost(raw, "the host"+why, "host %s%s", host, why))
	}
	return u, nil
}

// aboutTheHost is a sentence that names the host or the port of raw
// (fmt.Sprintf of format and args), or withheld with the reason they are not
// named. A value that holds an @ anywhere may carry user information url.Parse
// did not read as such: a password with a ?, a # or a / in it
// (http://pw?x@127.0.0.1:8888, http://pw/x@127.0.0.1:8888) leaves its head
// where url.Parse sees a host and a port. Hiding too much costs nothing, one
// sentence that shows a password is a leak (T-450 review #2 Mi-R2-1, T-451
// review #1 Ma-1).
func aboutTheHost(raw, withheld, format string, args ...any) string {
	if strings.Contains(raw, "@") {
		return withheld + " (the host and the port are not printed: the value holds an @, and what stands in " +
			"front of it may be a key)"
	}
	return fmt.Sprintf(format, args...)
}

// authority is the part of raw between :// and the first /, ? or #.
func authority(raw string) string {
	_, rest, _ := strings.Cut(raw, "://")
	if i := strings.IndexAny(rest, "/?#"); i >= 0 {
		return rest[:i]
	}
	return rest
}

// printableASCII is llm_not_printable_ascii of the scripts, negated: every byte
// in 0x20-0x7E.
func printableASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] < 0x20 || s[i] > 0x7e {
			return false
		}
	}
	return true
}

// needsExplicitPort is true for every local host (C-15 v1.3): loopback, a
// service of compose, an address of the LAN alike. A cloud address keeps the
// default of the scheme — there 443 is what the vendor documents.
func needsExplicitPort(host string) bool {
	return isLocalHost(canonicalHost(host))
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
