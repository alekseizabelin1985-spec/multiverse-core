// Package env is the single reader of the process environment.
//
// Every variable the platform reads is declared once, at package level, with
// its default, its documentation and its flags:
//
//	var brokers = env.Declare("MV_KAFKA_BROKERS", "127.0.0.1:19092", "bus brokers")
//
// The declaration is what makes the variable exist: Manifest returns the whole
// set, CheckExample compares it with .env.example in both directions, and
// Validate refuses to start a process whose required variables are missing
// (NFR-074, infrastructure.md §4.1, §4.2). The linter forbids os.Getenv
// outside this package, so nothing can be read past the manifest.
//
// Values of variables marked secret never appear in an error or a log line
// (ADR-009 p. 4): the reason is reported, the value is not.
package env

import (
	"errors"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Prefix is required on every platform variable (contracts.md §16 p. 5).
// Variables read by third-party images keep the names their image dictates and
// are declared with DeclareExternal.
const Prefix = "MV_"

// Scope says who reads a variable.
type Scope string

const (
	// ScopeProcess is read by a platform process (core, gateway, memory,
	// telegram-bot) or by mvctl. Validate checks it.
	ScopeProcess Scope = "process"
	// ScopeTooling is read outside a platform process — by a host script
	// (scripts/llm-server.*), by docker compose or by a third-party image —
	// and never reaches a container, so a process must not fail when it is
	// unset (infrastructure.md v0.3 §4.2 p. 5). A conditional requirement
	// still applies: it is a rule of the deployment, not of the process.
	ScopeTooling Scope = "tooling"
)

// ValueKind is the type a value is parsed as. It is what lets Validate refuse
// MV_SNAPSHOT_EVERY_FACTS=abc where the manifest is read — in mvctl env check
// or at start — instead of at the first Int() call deep inside a running
// process (NFR-074, infrastructure.md §4.2).
type ValueKind string

const (
	// KindString accepts any value and is the default.
	KindString ValueKind = "string"
	// KindInt is parsed by strconv.Atoi.
	KindInt ValueKind = "int"
	// KindBool is parsed by strconv.ParseBool.
	KindBool ValueKind = "bool"
	// KindDuration is parsed by time.ParseDuration.
	KindDuration ValueKind = "duration"
)

// Var is one declared variable. The zero value is not usable: a Var is always
// the result of Declare or DeclareExternal.
type Var struct {
	name     string
	def      string
	doc      string
	secret   bool
	required bool
	scope    Scope
	external bool
	kind     ValueKind
	enum     []string
	when     *condition
}

// condition makes a variable required only for some values of another one:
// MV_OLLAMA_URL matters only when MV_LLM_PROVIDER is ollama (ADR-005 add. 2
// p. 2).
type condition struct {
	name   string
	values []string
}

// Name returns the environment variable name.
func (v Var) Name() string { return v.name }

// Default returns the value used when the variable is unset or empty.
func (v Var) Default() string { return v.def }

// Doc returns the one-line description shown by mvctl env check.
func (v Var) Doc() string { return v.doc }

// IsSecret reports whether the value must never be printed or logged.
func (v Var) IsSecret() bool { return v.secret }

// IsRequired reports whether the variable must carry a value unconditionally.
func (v Var) IsRequired() bool { return v.required }

// IsExternal reports whether the variable belongs to a third-party image and
// therefore carries no MV_ prefix.
func (v Var) IsExternal() bool { return v.external }

// Scope returns who reads the variable.
func (v Var) Scope() Scope { return v.scope }

// Enum returns the allowed values, or nil when any value is accepted.
func (v Var) Enum() []string { return append([]string(nil), v.enum...) }

// Kind returns the declared type of the value; an undeclared one is a string.
func (v Var) Kind() ValueKind {
	if v.kind == "" {
		return KindString
	}
	return v.kind
}

// RequiredWhen returns the variable and the values that make this one
// required, and whether such a condition was declared.
func (v Var) RequiredWhen() (name string, values []string, ok bool) {
	if v.when == nil {
		return "", nil, false
	}
	return v.when.name, append([]string(nil), v.when.values...), true
}

// Option customises a declaration.
type Option func(*Var)

// Secret marks a variable whose value must stay out of errors, logs and
// .env.example (ADR-009 p. 4).
func Secret() Option { return func(v *Var) { v.secret = true } }

// Required marks a variable that must carry a value: a process whose required
// variable is empty fails at start rather than degrading silently (SEC-14).
func Required() Option { return func(v *Var) { v.required = true } }

// RequiredWhen marks a variable required only while another one holds one of
// the given values.
func RequiredWhen(name string, values ...string) Option {
	return func(v *Var) { v.when = &condition{name: name, values: values} }
}

// Tooling marks a variable read by a host script instead of a process.
func Tooling() Option { return func(v *Var) { v.scope = ScopeTooling } }

// OneOf restricts the value to a closed set; an unlisted value is an error at
// validation, not a surprise at the first use.
func OneOf(values ...string) Option {
	return func(v *Var) { v.enum = values }
}

// Kind declares the type of the value. Validate then parses the value the same
// way Int, Bool or Duration will, so a value that cannot be read is reported
// by mvctl env check rather than by whichever call site happens to touch the
// variable first.
func Kind(k ValueKind) Option { return func(v *Var) { v.kind = k } }

// IsInt is the short form of Kind(KindInt).
func IsInt() Option { return Kind(KindInt) }

// IsBool is the short form of Kind(KindBool).
func IsBool() Option { return Kind(KindBool) }

// IsDuration is the short form of Kind(KindDuration).
func IsDuration() Option { return Kind(KindDuration) }

var registry = struct {
	mu         sync.RWMutex
	vars       map[string]Var
	order      []string
	deprecated map[string]string
}{
	vars:       make(map[string]Var),
	deprecated: make(map[string]string),
}

// Declare registers a platform variable. It panics on a name without the MV_
// prefix, on an empty description and on a second declaration of the same
// name: all three are programming errors visible when the package is loaded,
// long before a process serves anything.
func Declare(name, def, doc string, opts ...Option) Var {
	if !strings.HasPrefix(name, Prefix) {
		panic(fmt.Sprintf("env: platform variable %q must start with %s; use DeclareExternal for a variable of a third-party image", name, Prefix))
	}
	return declare(name, def, doc, false, opts)
}

// DeclareExternal registers a variable whose name is dictated by a third-party
// image (MINIO_ROOT_USER, NEO4J_PASSWORD, OLLAMA_*, COMPOSE_*). It panics on a
// name carrying the platform prefix.
func DeclareExternal(name, def, doc string, opts ...Option) Var {
	if strings.HasPrefix(name, Prefix) {
		panic(fmt.Sprintf("env: %q carries the platform prefix %s; declare it with Declare", name, Prefix))
	}
	return declare(name, def, doc, true, opts)
}

// DeclareDeprecated records a variable that must no longer be used. It is
// absent from the manifest and from .env.example; mvctl env check reports it
// when it is still present in an environment (infrastructure.md v0.3 §4.2).
func DeclareDeprecated(name, reason string) {
	if reason == "" {
		panic(fmt.Sprintf("env: deprecated variable %q needs a reason", name))
	}
	registry.mu.Lock()
	defer registry.mu.Unlock()
	if _, dup := registry.deprecated[name]; dup {
		panic(fmt.Sprintf("env: variable %q declared deprecated twice", name))
	}
	if _, live := registry.vars[name]; live {
		panic(fmt.Sprintf("env: variable %q is both declared and deprecated", name))
	}
	registry.deprecated[name] = reason
}

func declare(name, def, doc string, external bool, opts []Option) Var {
	if !validName(name) {
		panic(fmt.Sprintf("env: %q is not a valid variable name (A-Z, 0-9 and _, starting with a letter)", name))
	}
	if strings.TrimSpace(doc) == "" {
		panic(fmt.Sprintf("env: variable %q needs a description: it is what mvctl env check and .env.example show", name))
	}
	v := Var{name: name, def: def, doc: doc, scope: ScopeProcess, external: external}
	for _, opt := range opts {
		opt(&v)
	}
	if v.required && def != "" {
		panic(fmt.Sprintf("env: variable %q is required and has default %q; a default means it is not required", name, def))
	}
	if def != "" && v.checkKind(def) != nil {
		panic(fmt.Sprintf("env: default %q of variable %q is not a valid %s", def, name, v.Kind()))
	}
	registry.mu.Lock()
	defer registry.mu.Unlock()
	if _, dup := registry.vars[name]; dup {
		panic(fmt.Sprintf("env: variable %q declared twice", name))
	}
	if _, gone := registry.deprecated[name]; gone {
		panic(fmt.Sprintf("env: variable %q is declared deprecated", name))
	}
	registry.vars[name] = v
	registry.order = append(registry.order, name)
	return v
}

func validName(name string) bool {
	if name == "" || name[0] < 'A' || name[0] > 'Z' {
		return false
	}
	for i := 0; i < len(name); i++ {
		c := name[i]
		switch {
		case c >= 'A' && c <= 'Z', c >= '0' && c <= '9', c == '_':
		default:
			return false
		}
	}
	return true
}

// Manifest returns every declared variable, sorted by name. It is the source
// for the comparison with .env.example and for mvctl env check.
func Manifest() []Var {
	registry.mu.RLock()
	defer registry.mu.RUnlock()
	out := make([]Var, 0, len(registry.vars))
	for _, name := range registry.order {
		out = append(out, registry.vars[name])
	}
	sort.Slice(out, func(i, j int) bool { return out[i].name < out[j].name })
	return out
}

// Lookup returns the declaration of name.
func Lookup(name string) (Var, bool) {
	registry.mu.RLock()
	defer registry.mu.RUnlock()
	v, ok := registry.vars[name]
	return v, ok
}

// DeprecatedNames returns the retired variable names with their reasons.
func DeprecatedNames() map[string]string {
	registry.mu.RLock()
	defer registry.mu.RUnlock()
	out := make(map[string]string, len(registry.deprecated))
	for name, reason := range registry.deprecated {
		out[name] = reason
	}
	return out
}

// Source resolves a variable name to its raw value. OS reads the process
// environment; a parsed .env file is a Source too, which is what lets
// mvctl env check validate a file the process never loaded.
type Source func(name string) (string, bool)

// OS returns the Source backed by the process environment.
func OS() Source { return os.LookupEnv }

// MapSource returns a Source backed by m.
func MapSource(m map[string]string) Source {
	return func(name string) (string, bool) {
		val, ok := m[name]
		return val, ok
	}
}

// String returns the value from the process environment, or the default when
// the variable is unset. Surrounding whitespace is dropped: a value pasted
// into .env with a trailing space is the same value.
//
// A variable set to an empty value returns that empty value, not the default:
// MV_CORE_ADMIN_CLIENTS= means "no client is admitted", and an operator who
// empties an allow-list to close a route must not be handed the shipped list
// back (review T-007 Mi-6). Unset and set-to-empty are therefore different
// things, and Validate reports an empty required variable either way.
func (v Var) String() string { return v.StringFrom(nil) }

// StringFrom is String against an explicit source; a nil source is the process
// environment.
func (v Var) StringFrom(src Source) string {
	if raw, ok := v.rawFrom(src); ok {
		return raw
	}
	return v.def
}

// Set reports whether the variable carries a non-empty value in the process
// environment. A variable set to an empty value is not set in this sense: it
// carries nothing, it only keeps the default from applying.
func (v Var) Set() bool {
	raw, ok := v.rawFrom(nil)
	return ok && raw != ""
}

// rawFrom returns the trimmed value and whether the source carries the
// variable at all. Present-and-empty is reported as present, which is what
// makes an empty value beat a non-empty default.
func (v Var) rawFrom(src Source) (string, bool) {
	if src == nil {
		src = OS()
	}
	raw, ok := src(v.name)
	if !ok {
		return "", false
	}
	return strings.TrimSpace(raw), true
}

// List splits a comma separated value, dropping empty entries. It is how every
// allow-list of the platform is written (MV_CORE_ADMIN_CLIENTS,
// MV_TELEGRAM_ALLOWED_USER_IDS, MV_KAFKA_BROKERS). Setting the variable to an
// empty value yields an empty list, which for an allow-list means nobody —
// see String.
func (v Var) List() []string { return v.ListFrom(nil) }

// ListFrom is List against an explicit source.
func (v Var) ListFrom(src Source) []string {
	raw := v.StringFrom(src)
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// Int parses the value as a decimal integer.
func (v Var) Int() (int, error) { return v.IntFrom(nil) }

// IntFrom is Int against an explicit source.
func (v Var) IntFrom(src Source) (int, error) {
	raw := v.StringFrom(src)
	n, err := strconv.Atoi(raw)
	if err != nil {
		return 0, v.errf(raw, "expected an integer")
	}
	return n, nil
}

// Bool parses true/false and the other forms strconv.ParseBool accepts.
func (v Var) Bool() (bool, error) { return v.BoolFrom(nil) }

// BoolFrom is Bool against an explicit source.
func (v Var) BoolFrom(src Source) (bool, error) {
	raw := v.StringFrom(src)
	b, err := strconv.ParseBool(raw)
	if err != nil {
		return false, v.errf(raw, "expected true or false")
	}
	return b, nil
}

// Duration parses a Go duration such as 25s or 100ms.
func (v Var) Duration() (time.Duration, error) { return v.DurationFrom(nil) }

// DurationFrom is Duration against an explicit source.
func (v Var) DurationFrom(src Source) (time.Duration, error) {
	raw := v.StringFrom(src)
	d, err := time.ParseDuration(raw)
	if err != nil {
		return 0, v.errf(raw, "expected a duration such as 25s or 100ms")
	}
	return d, nil
}

// Error reports a variable that carries no acceptable value. The value of a
// secret is never part of the message (ADR-009 p. 4).
type Error struct {
	Name   string
	Reason string
	// Value is the offending value, empty for a variable declared secret.
	Value string
	// Secret says the value was withheld rather than absent.
	Secret bool
}

// Error implements error.
func (e *Error) Error() string {
	switch {
	case e.Secret:
		return fmt.Sprintf("%s: %s (value withheld: the variable is a secret)", e.Name, e.Reason)
	case e.Value == "":
		return fmt.Sprintf("%s: %s", e.Name, e.Reason)
	default:
		return fmt.Sprintf("%s=%q: %s", e.Name, e.Value, e.Reason)
	}
}

func (v Var) errf(value, format string, args ...any) error {
	err := &Error{Name: v.name, Reason: fmt.Sprintf(format, args...), Secret: v.secret}
	if !v.secret {
		err.Value = value
	}
	return err
}

// Validate checks the declared variables of src: a required one carries a
// value, a conditionally required one carries a value while its condition
// holds, a value outside a declared enum is refused, and a value that does not
// parse as its declared kind is refused with the error its typed getter would
// otherwise have raised at the first use. The unconditional
// requirement does not apply to tooling variables — a container has no reason
// to know the path of llama-server on the host. A nil source is the process
// environment.
//
// Errors are joined, so one run reports every problem instead of the first.
func Validate(src Source) error {
	var problems []error
	for _, v := range Manifest() {
		if err := v.validate(src); err != nil {
			problems = append(problems, err)
		}
	}
	return errors.Join(problems...)
}

func (v Var) validate(src Source) error {
	raw, present := v.rawFrom(src)
	if !present {
		raw = v.def
	}
	if raw == "" {
		if v.required && v.scope != ScopeTooling {
			return v.errf("", "required and empty")
		}
		if name, values, ok := v.RequiredWhen(); ok && conditionMet(src, name, values) {
			return v.errf("", "required while %s is %s", name, strings.Join(values, " or "))
		}
		return nil
	}
	if len(v.enum) > 0 && !contains(v.enum, raw) {
		return v.errf(raw, "expected one of %s", strings.Join(v.enum, ", "))
	}
	return v.checkKind(raw)
}

// checkKind parses raw the way the typed getter of the declared kind will and
// reports the same error it would. An empty raw never reaches here: an unset
// variable is a question of requirement, not of type.
func (v Var) checkKind(raw string) error {
	switch v.kind {
	case KindInt:
		if _, err := strconv.Atoi(raw); err != nil {
			return v.errf(raw, "expected an integer")
		}
	case KindBool:
		if _, err := strconv.ParseBool(raw); err != nil {
			return v.errf(raw, "expected true or false")
		}
	case KindDuration:
		if _, err := time.ParseDuration(raw); err != nil {
			return v.errf(raw, "expected a duration such as 25s or 100ms")
		}
	}
	return nil
}

func conditionMet(src Source, name string, values []string) bool {
	other, ok := Lookup(name)
	if !ok {
		return false
	}
	return contains(values, other.StringFrom(src))
}

func contains(values []string, want string) bool {
	for _, v := range values {
		if v == want {
			return true
		}
	}
	return false
}

// Deprecated returns the retired variables still present in src, each with the
// reason it was retired. It reports names, never values.
func Deprecated(src Source) []string {
	if src == nil {
		src = OS()
	}
	var found []string
	for name, reason := range DeprecatedNames() {
		if _, ok := src(name); ok {
			found = append(found, fmt.Sprintf("%s: %s", name, reason))
		}
	}
	sort.Strings(found)
	return found
}
