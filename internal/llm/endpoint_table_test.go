package llm

// The third test of parity of the rule "local address" (C-15 v1.3, T-451). The
// rule has one table of cases, testdata/llm/local-endpoints.tsv (T-450); the
// parity stand holds bash and PowerShell to it (T01, T02 of
// testdata/script-parity), and this file holds the platform: IsLocalEndpoint,
// LoadConfig and the cloud gate answer every row as the table does. Where the
// text of the contract and a row disagree, the row is right — so the table is
// read, not copied here.

import (
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"multiverse-core.io/shared/env"
)

const (
	endpointTable = "../../testdata/llm/local-endpoints.tsv"
	// endpointStand holds the cases the format of the table cannot: a URL with a
	// space. The stand is package main, so its literal is read from the source.
	endpointStand = "../../testdata/script-parity/endpoints.go"
	// endpointTableMinRows is the floor of the stand (endpointTableMinRows of
	// testdata/script-parity): a table that lost its rows would let every
	// implementation pass.
	endpointTableMinRows = 40
)

const (
	classLocal   = "local"
	classCloud   = "cloud"
	classInvalid = "invalid"
)

type endpointRow struct {
	where, url, want, reason string
}

// readEndpointTable reads the table in the format its header describes. The
// \r at the end of a line is dropped: a checkout without the .gitattributes of
// this branch turns LF into CRLF, and "local\r" is no answer.
func readEndpointTable(t *testing.T) []endpointRow {
	t.Helper()
	raw, err := os.ReadFile(filepath.FromSlash(endpointTable))
	if err != nil {
		t.Fatalf("read the table: %v", err)
	}
	var (
		rows   []endpointRow
		header bool
		seen   = map[string]string{}
		count  = map[string]int{}
	)
	for i, line := range strings.Split(string(raw), "\n") {
		where := endpointTable + ":" + strconv.Itoa(i+1)
		line = strings.TrimSuffix(line, "\r")
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Split(line, "\t")
		if !header {
			header = true
			if line != "url\tanswer\treason" {
				t.Fatalf("%s: the header must be url<TAB>answer<TAB>reason, got %q", where, line)
			}
			continue
		}
		if len(fields) != 3 {
			t.Fatalf("%s: %d fields, want 3 separated by one TAB: %q", where, len(fields), line)
		}
		r := endpointRow{where: where, url: fields[0], want: fields[1], reason: fields[2]}
		switch r.want {
		case classLocal, classCloud, classInvalid:
		default:
			t.Fatalf("%s: answer %q is not local, cloud or invalid", where, r.want)
		}
		if r.url == "" || strings.TrimSpace(r.reason) == "" {
			t.Fatalf("%s: a row needs a URL and a reason: %q", where, line)
		}
		if prev, dup := seen[r.url]; dup {
			t.Fatalf("%s: %s repeats %s", where, r.url, prev)
		}
		seen[r.url] = where
		count[r.want]++
		rows = append(rows, r)
	}
	if len(rows) < endpointTableMinRows {
		t.Fatalf("the table holds %d cases, at least %d are expected", len(rows), endpointTableMinRows)
	}
	for _, class := range []string{classLocal, classCloud, classInvalid} {
		if count[class] == 0 {
			t.Fatalf("no case of the table answers %s", class)
		}
	}
	return rows
}

// readEndpointStandCases takes the url and want of every element of the
// endpointStandCases literal of the stand.
func readEndpointStandCases(t *testing.T) []endpointRow {
	t.Helper()
	file, err := parser.ParseFile(token.NewFileSet(), filepath.FromSlash(endpointStand), nil, 0)
	if err != nil {
		t.Fatalf("parse the stand: %v", err)
	}
	var lit *ast.CompositeLit
	ast.Inspect(file, func(n ast.Node) bool {
		spec, ok := n.(*ast.ValueSpec)
		if !ok || len(spec.Names) != 1 || spec.Names[0].Name != "endpointStandCases" || len(spec.Values) != 1 {
			return true
		}
		lit, _ = spec.Values[0].(*ast.CompositeLit)
		return false
	})
	if lit == nil {
		t.Fatalf("%s: no composite literal endpointStandCases", endpointStand)
	}
	var rows []endpointRow
	for i, elt := range lit.Elts {
		r := endpointRow{where: endpointStand + ": endpointStandCases[" + strconv.Itoa(i) + "]"}
		fields, ok := elt.(*ast.CompositeLit)
		if !ok {
			t.Fatalf("%s: not a composite literal", r.where)
		}
		for _, f := range fields.Elts {
			kv, ok := f.(*ast.KeyValueExpr)
			if !ok {
				t.Fatalf("%s: a field without a key", r.where)
			}
			key, _ := kv.Key.(*ast.Ident)
			val, _ := kv.Value.(*ast.BasicLit)
			if key == nil || val == nil || val.Kind != token.STRING {
				t.Fatalf("%s: a field that is not key: \"string\"", r.where)
			}
			s, err := strconv.Unquote(val.Value)
			if err != nil {
				t.Fatalf("%s: %v", r.where, err)
			}
			switch key.Name {
			case "url":
				r.url = s
			case "want":
				r.want = s
			case "reason":
				r.reason = s
			}
		}
		if r.url == "" || r.want == "" {
			t.Fatalf("%s: a case needs url and want", r.where)
		}
		rows = append(rows, r)
	}
	if len(rows) == 0 {
		t.Fatalf("%s: endpointStandCases is empty", endpointStand)
	}
	return rows
}

// classOf reads the answer of IsLocalEndpoint as a class of the table, and
// fails a refusal that is not a configuration error or that is the refusal of
// the cloud gate.
func classOf(ok bool, err error) (string, error) {
	switch {
	case err == nil && ok:
		return classLocal, nil
	case err == nil:
		return classCloud, nil
	case errors.Is(err, ErrConfig) && !errors.Is(err, ErrCloudDisabled):
		return classInvalid, nil
	}
	return "", err
}

// hiddenParts are the parts of a value no sentence about it may print:
// everything from the first ? or #, and the user information — all that stands
// between :// and the last @ of the value, wherever that @ is. A password with
// a ?, a # or a / in it leaves its head where url.Parse sees a host and a port
// (http://pw?x@127.0.0.1:8888 has host pw), so that head, and its host and
// port apart, are hidden parts too (T-451 review #1 Ma-1). A piece shorter
// than three characters is not looked for: it would be found in any sentence.
func hiddenParts(raw string) []string {
	var parts []string
	add := func(p string) {
		if len(p) >= 3 {
			parts = append(parts, p)
		}
	}
	if i := strings.IndexAny(raw, "?#"); i >= 0 {
		add(raw[i:])
	}
	_, after, ok := strings.Cut(raw, "://")
	if !ok {
		return parts
	}
	at := strings.LastIndex(after, "@")
	if at <= 0 {
		return parts
	}
	user := after[:at]
	add(user)
	if i := strings.IndexAny(user, "/?#"); i > 0 {
		head := user[:i]
		add(head)
		if h, port, cut := strings.Cut(head, ":"); cut {
			add(h)
			add(port)
		}
	}
	return parts
}

func TestIsLocalEndpointAnswersTheTable(t *testing.T) {
	rows := readEndpointTable(t)
	stand := readEndpointStandCases(t)
	t.Logf("%d cases of the table, %d of the stand", len(rows), len(stand))

	for _, r := range append(rows, stand...) {
		got, err := classOf(IsLocalEndpoint(r.url))
		if err != nil {
			t.Errorf("%s: IsLocalEndpoint(%q): an error that is not a configuration error: %v", r.where, r.url, err)
			continue
		}
		if got != r.want {
			_, why := IsLocalEndpoint(r.url)
			t.Errorf("%s: IsLocalEndpoint(%q) is %s, the table says %s (%s); error: %v", r.where, r.url, got, r.want, r.reason, why)
		}
	}
}

// Every row through the path the platform takes: LoadConfig, then the gate
// providers.New calls, for both addresses the gate reads, and the gate on a
// Config assembled by hand from the raw value.
func TestTheCloudGateAnswersTheTable(t *testing.T) {
	for _, r := range append(readEndpointTable(t), readEndpointStandCases(t)...) {
		for _, v := range []struct{ provider, name string }{
			{ProviderOpenAICompat, env.LLMURL.Name()},
			{ProviderOllama, env.OllamaURL.Name()},
		} {
			check := func(how string, err error, cloudEnabled bool) {
				t.Helper()
				want := r.want
				if want == classCloud && cloudEnabled {
					want = classLocal // the flag lets it through
				}
				switch {
				case want == classLocal && err != nil:
					t.Errorf("%s: %s %s=%q: %v, the table says %s", r.where, how, v.name, r.url, err, r.want)
				case want == classCloud && !errors.Is(err, ErrCloudDisabled):
					t.Errorf("%s: %s %s=%q: err = %v, want ErrCloudDisabled (the table says cloud)", r.where, how, v.name, r.url, err)
				case want == classInvalid && (!errors.Is(err, ErrConfig) || errors.Is(err, ErrCloudDisabled)):
					t.Errorf("%s: %s %s=%q: err = %v, want a configuration error that is not ErrCloudDisabled (the table says invalid)", r.where, how, v.name, r.url, err)
				}
				if err != nil {
					if !strings.Contains(err.Error(), v.name) {
						t.Errorf("%s: %s %s=%q: the error does not name the variable: %v", r.where, how, v.name, r.url, err)
					}
					for _, part := range hiddenParts(r.url) {
						if strings.Contains(err.Error(), part) {
							t.Errorf("%s: %s %s=%q: the error prints %q: %v", r.where, how, v.name, r.url, part, err)
						}
					}
				}
			}

			for _, cloud := range []string{"false", "true"} {
				cfg, err := LoadConfig(source("MV_LLM_PROVIDER", v.provider, v.name, r.url, "MV_LLM_CLOUD_ENABLED", cloud))
				if err == nil {
					err = cfg.CheckCloudGate()
				}
				check("LoadConfig and the gate, MV_LLM_CLOUD_ENABLED="+cloud+",", err, cloud == "true")
			}

			hand := Config{Provider: v.provider}
			if v.provider == ProviderOllama {
				hand.OllamaURL = r.url
			} else {
				hand.URL = r.url
			}
			check("the gate on a hand-made Config,", hand.CheckCloudGate(), false)
		}
	}
}

// No sentence of a refusal prints the user information or the query of the
// value (T-450 review #2 Mi-R2-1, the same rule as the scripts). The error of
// url.Parse quotes the value whole, and a password with a / in it is what
// url.Parse reports as a port: every path of the refusal is tried with the
// fictitious password FAKEPW123.
func TestARefusalNeverPrintsTheUserInformationOrTheQuery(t *testing.T) {
	const secret = "FAKEPW123"
	values := []string{
		"http://user:FAKEPW123@127.0.0.1:8888",
		"http://FAKEPW123@127.0.0.1:8888",
		"http://FAKEPW123:x@127.0.0.1:8888",
		"http://u:FAKEPW123@127.0.0.1",
		"http://u:FAKEPW123@0.0.0.0:8888",
		"http://u:FAKEPW123@",
		"http://u:FAKEPW123/x@127.0.0.1:8888",
		"http://u:FAKEPW123?x@127.0.0.1:8888",
		"http://u:FAKEPW123#x@127.0.0.1:8888",
		"http://u:FAKEPW123@[::1:8888",
		"http://u:FAKEPW123@local host:8888",
		"http://u:FAKEPW123@ol%61ma:11434",
		"http://u:FAKEPW123@127.0.0.1:0",
		"ftp://u:FAKEPW123@127.0.0.1:21",
		"u:FAKEPW123@127.0.0.1:8888",
		"http://127.0.0.1:8888/v1?api_key=FAKEPW123",
		"http://127.0.0.1:8888#FAKEPW123",
		"http://127.0.0.1:FAKEPW123",
		"http://[fe80::1%25FAKEPW123]:8080",
		// T-451 review #1 Ma-1: the password carries a ?, a # or a /, and
		// url.Parse reads its head as the host — or as the host and the port.
		"http://FAKEPW123?x@127.0.0.1:8888",
		"http://FAKEPW123#x@127.0.0.1:8888",
		"http://FAKEPW123?@api.openai.com",
		"http://fakepw123/x@127.0.0.1:8888",
		"http://FAKEPW123:/x@127.0.0.1:8888",
		"http://FAKEPW123:99999/x@127.0.0.1:8888",
		"http://u:99999/FAKEPW123@127.0.0.1:8888",
		"http://FAKEPW123/x@0.0.0.0:8888",
		"http://FAKEPW123.example/x@127.0.0.1:8888?",
		"http://FAKEPW123\u00a0@127.0.0.1:8888",
		"http://FAKEPW123%C3%A9x@127.0.0.1:8888",
	}
	for _, raw := range values {
		errs := map[string]error{}
		_, errs["IsLocalEndpoint"] = IsLocalEndpoint(raw)
		_, errs["LoadConfig MV_LLM_URL"] = LoadConfig(source(env.LLMURL.Name(), raw))
		_, errs["LoadConfig MV_OLLAMA_URL"] = LoadConfig(source("MV_LLM_PROVIDER", ProviderOllama, env.OllamaURL.Name(), raw))
		errs["the gate"] = Config{Provider: ProviderOpenAICompat, URL: raw}.CheckCloudGate()
		for how, err := range errs {
			if !errors.Is(err, ErrConfig) || errors.Is(err, ErrCloudDisabled) {
				t.Errorf("%s(%q): err = %v, want a configuration error", how, raw, err)
				continue
			}
			if strings.Contains(strings.ToUpper(err.Error()), secret) {
				t.Errorf("%s(%q): the error prints the password: %v", how, raw, err)
			}
		}
	}

	// The same forms where url.Parse sees a host in the cloud: parseEndpoint
	// accepts them, the gate refuses them, and its sentence does not name the
	// head of the password.
	for _, raw := range []string{
		"http://FAKEPW123.example/x@127.0.0.1:8888",
		"https://FAKEPW123.example:443/v1@api.openai.com",
	} {
		if ok, err := IsLocalEndpoint(raw); err != nil || ok {
			t.Fatalf("IsLocalEndpoint(%q) = %v, %v; this form is meant to be the cloud", raw, ok, err)
		}
		for how, cfg := range map[string]Config{
			"MV_LLM_URL":    {Provider: ProviderOpenAICompat, URL: raw},
			"MV_OLLAMA_URL": {Provider: ProviderOllama, OllamaURL: raw},
		} {
			err := cfg.CheckCloudGate()
			if !errors.Is(err, ErrCloudDisabled) {
				t.Errorf("the gate on %s=%q: err = %v, want ErrCloudDisabled", how, raw, err)
				continue
			}
			if strings.Contains(strings.ToUpper(err.Error()), secret) {
				t.Errorf("the gate on %s=%q prints the password: %v", how, raw, err)
			}
		}
	}
}

// An address outside printable ASCII is invalid, as the scripts answer it
// (C-15; the decision of the orchestrator on T-451 review #1 Mi-1): an escape
// url.Parse would turn into a letter beyond ASCII, a host that is not ASCII
// (a full-width localhost, an ideographic full stop, a soft hyphen, a letter
// of another script that looks like o), and a Unicode space at an edge, which
// strings.TrimSpace would have removed. ASCII whitespace at the edges is still
// trimmed. Every path of the platform gives the same answer.
func TestAnAddressOutsidePrintableASCIIIsInvalid(t *testing.T) {
	invalid := []string{
		"http://ol%C3%A9ma:8080",
		"http://ollama%C3%A9:8080/v1",
		"http://\uff4c\uff4f\uff43\uff41\uff4c\uff48\uff4f\uff53\uff54:8080",
		"http://127\u30020\u30020\u30021:8080",
		"http://localhost\u3002:8080",
		"http://local\u00adhost:8080",
		"http://l\u0585calhost:8080",
		"http://ol\u00e9ma:8080",
		"\u00a0http://127.0.0.1:8080\u00a0",
		"http://127.0.0.1:8080\u00a0",
		"\u2003http://127.0.0.1:8080",
		"http://127.0.0.1:8080\u3000",
		"http://127.0.0.1:8080\u0085",
		"\u00a0http://ollama:11434",
		"http://127.0.0.1\u200b:8080",
		"\ufeffhttp://127.0.0.1:8080",
	}
	for _, raw := range invalid {
		errs := map[string]error{}
		_, errs["IsLocalEndpoint"] = IsLocalEndpoint(raw)
		for _, cloud := range []string{"false", "true"} {
			_, errs["LoadConfig MV_LLM_URL, cloud "+cloud] = LoadConfig(source(
				env.LLMURL.Name(), raw, "MV_LLM_CLOUD_ENABLED", cloud))
			_, errs["LoadConfig MV_OLLAMA_URL, cloud "+cloud] = LoadConfig(source(
				"MV_LLM_PROVIDER", ProviderOllama, env.OllamaURL.Name(), raw, "MV_LLM_CLOUD_ENABLED", cloud))
		}
		errs["the gate, MV_LLM_URL"] = Config{Provider: ProviderOpenAICompat, URL: raw}.CheckCloudGate()
		errs["the gate, MV_OLLAMA_URL"] = Config{Provider: ProviderOllama, OllamaURL: raw}.CheckCloudGate()
		errs["the gate with the cloud enabled"] = Config{Provider: ProviderOpenAICompat, URL: raw,
			Cloud: Cloud{Enabled: true}}.CheckCloudGate()
		for how, err := range errs {
			if !errors.Is(err, ErrConfig) || errors.Is(err, ErrCloudDisabled) {
				t.Errorf("%s(%q): err = %v, want a configuration error that is not the cloud refusal", how, raw, err)
				continue
			}
			if strings.Contains(err.Error(), strings.TrimSpace(raw)) {
				t.Errorf("%s(%q): the error prints the value: %v", how, raw, err)
			}
		}
	}

	for _, raw := range []string{" \thttp://127.0.0.1:8080\r\n", "\v\fhttp://ollama:11434/v1\f\v "} {
		if ok, err := IsLocalEndpoint(raw); err != nil || !ok {
			t.Errorf("IsLocalEndpoint(%q) = %v, %v; ASCII whitespace at the edges is trimmed, want local", raw, ok, err)
		}
		cfg, err := LoadConfig(source(env.LLMURL.Name(), raw))
		if err == nil {
			err = cfg.CheckCloudGate()
		}
		if err != nil {
			t.Errorf("LoadConfig and the gate, MV_LLM_URL=%q: %v", raw, err)
		}
	}
}

// canonicalHost of its own result is that result, so the gate and Cost, which
// canonicalizes the host EndpointHost has already canonicalized, read one
// class (T-451 review #1 Mi-2).
func TestCanonicalHostIsIdempotent(t *testing.T) {
	hosts := []string{
		"localhost..", "api.localhost..", "host.docker.internal..", "127.0.0.1..", "ollama..", "a.b..",
		"LOCALHOST.", "Api.Example.COM.", "127.0.0.1.", "ollama.", ".", "..", "::FFFF:10.0.0.1",
	}
	for _, r := range append(readEndpointTable(t), readEndpointStandCases(t)...) {
		if u, err := url.Parse(r.url); err == nil {
			hosts = append(hosts, u.Hostname())
		}
	}
	for _, h := range hosts {
		once := canonicalHost(h)
		if twice := canonicalHost(once); twice != once {
			t.Errorf("canonicalHost(%q) = %q, and canonicalHost of that = %q", h, once, twice)
		}
	}
}

// A name with an empty label at its end is the cloud to the gate, and it has
// no price: Cost of its EndpointHost is ErrNoPrice, not the 0 of a local
// address (T-451 review #1 Mi-2; C-07 limit_money).
func TestADoubleDotIsNeitherLocalNorFree(t *testing.T) {
	for _, raw := range []string{
		"http://localhost..:8080", "http://api.localhost..:8080", "http://host.docker.internal..:8080",
		"http://LOCALHOST..:8080/v1",
	} {
		if ok, err := IsLocalEndpoint(raw); err != nil || ok {
			t.Errorf("IsLocalEndpoint(%q) = %v, %v; want cloud", raw, ok, err)
		}
		if err := (Config{Provider: ProviderOpenAICompat, URL: raw}).CheckCloudGate(); !errors.Is(err, ErrCloudDisabled) {
			t.Errorf("the gate on %q: err = %v, want ErrCloudDisabled", raw, err)
		}
		host, err := EndpointHost(raw)
		if err != nil {
			t.Fatalf("EndpointHost(%q): %v", raw, err)
		}
		u, err := url.Parse(raw)
		if err != nil {
			t.Fatalf("url.Parse(%q): %v", raw, err)
		}
		for _, key := range []string{host, u.Host} {
			if cost, err := (*Prices)(nil).Cost(key, "m", Tokens{Prompt: 1000, Completion: 1000}); !errors.Is(err, ErrNoPrice) {
				t.Errorf("Cost(%q) of %q = %v, %v; want ErrNoPrice", key, raw, cost, err)
			}
		}
	}
}

// Go refuses http://::1:8888, and GODEBUG=urlstrictcolons=0 turns it into host
// ::1 and port 8888. The class of a value does not follow that switch.
func TestABareIPv6LiteralIsInvalidWhateverGODEBUGSays(t *testing.T) {
	t.Setenv("GODEBUG", "urlstrictcolons=0")
	const raw = "http://::1:8888"
	if u, err := url.Parse(raw); err != nil || u.Hostname() != "::1" { //nolint:staticcheck // SA1007: the value is invalid by default on purpose, the switch makes it parse
		t.Skipf("url.Parse(%q) with urlstrictcolons=0 = %v, %v: the switch no longer applies", raw, u, err)
	}
	if _, err := IsLocalEndpoint(raw); !errors.Is(err, ErrConfig) || !strings.Contains(err.Error(), "bare IPv6") {
		t.Fatalf("IsLocalEndpoint(%q) = %v, want the refusal of a bare IPv6 literal", raw, err)
	}
}
