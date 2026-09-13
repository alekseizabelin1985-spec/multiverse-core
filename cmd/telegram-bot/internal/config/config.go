// Package config reads the configuration of the Telegram bot from the
// manifest of shared/env (component §11.3, NFR-074). The bot reads nothing
// past it: every variable is an MV_TELEGRAM_* declaration of shared/env/vars.go.
//
// Config holds two secrets, the token and the key of action_key, and the
// allow-list, whose entries are Telegram user ids. None of them is part of an
// error or of the printed form of a Config.
package config

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"multiverse-core.io/shared/env"
)

// MinSaltLen is the shortest explicit key of action_key accepted. The key is
// what keeps action_key irreversible (ADR-018): a short one is guessed, and it
// is also too short for the log redactor to recognise.
const MinSaltLen = 16

// ErrTokenMissing is returned when MV_TELEGRAM_BOT_TOKEN is empty: the bot does
// not start without one (SEC-08).
var ErrTokenMissing = errors.New("MV_TELEGRAM_BOT_TOKEN is empty: the bot cannot start without a token from @BotFather")

// Config is the configuration of one bot process.
type Config struct {
	// Token is the bot token. It is a secret: never logged, never printed.
	Token string
	// AllowedUserIDs is the allow-list of Telegram user ids (SEC-06). Empty
	// means the bot admits nobody.
	AllowedUserIDs []int64
	// GatewayURL is the address of the gateway HTTP API.
	GatewayURL string
	// PollTimeout is the long polling timeout of getUpdates.
	PollTimeout time.Duration
	// HealthAddr is the listen address of /health of the bot.
	HealthAddr string
	// CommandsPerMinute is how many commands one user may send within any 60
	// seconds (SEC-11).
	CommandsPerMinute int
	// ActionKeySalt is the HMAC key of action_key: MV_TELEGRAM_ACTION_KEY_SALT,
	// or SHA-256 of the token when that is empty (component §10.3).
	ActionKeySalt []byte
	// saltExplicit says the salt came from the environment and is therefore
	// a secret of its own, not a one-way derivative of the token.
	saltExplicit bool
}

// Load reads the configuration from src; a nil src is the process
// environment. Every problem is reported at once.
func Load(src env.Source) (Config, error) {
	var (
		cfg      Config
		problems []error
	)

	cfg.Token = env.TelegramBotToken.StringFrom(src)
	switch {
	case cfg.Token == "":
		problems = append(problems, ErrTokenMissing)
	case !tokenShaped(cfg.Token):
		problems = append(problems, errors.New("MV_TELEGRAM_BOT_TOKEN does not look like a bot token (<digits>:<secret>); value withheld"))
	}

	ids, err := parseAllowList(env.TelegramAllowedUserIDs.ListFrom(src))
	if err != nil {
		problems = append(problems, err)
	}
	cfg.AllowedUserIDs = ids

	cfg.GatewayURL = strings.TrimRight(env.TelegramGatewayURL.StringFrom(src), "/")
	if u, err := url.Parse(cfg.GatewayURL); err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		problems = append(problems, fmt.Errorf("MV_TELEGRAM_GATEWAY_URL=%q: expected an http(s) URL with a host", printableURL(cfg.GatewayURL)))
	}

	if secs, err := env.TelegramPollTimeoutS.IntFrom(src); err != nil {
		problems = append(problems, err)
	} else if secs < 1 {
		problems = append(problems, fmt.Errorf("MV_TELEGRAM_POLL_TIMEOUT_S=%d: expected at least 1 second", secs))
	} else {
		cfg.PollTimeout = time.Duration(secs) * time.Second
	}

	cfg.HealthAddr = env.TelegramHealthAddr.StringFrom(src)

	if n, err := env.TelegramCommandsPerMin.IntFrom(src); err != nil {
		problems = append(problems, err)
	} else if n < 1 {
		problems = append(problems, fmt.Errorf("MV_TELEGRAM_COMMANDS_PER_MIN=%d: expected at least 1", n))
	} else {
		cfg.CommandsPerMinute = n
	}

	if salt := env.TelegramActionKeySalt.StringFrom(src); salt != "" {
		if utf8.RuneCountInString(salt) < MinSaltLen {
			problems = append(problems, fmt.Errorf("MV_TELEGRAM_ACTION_KEY_SALT is shorter than %d characters; value withheld", MinSaltLen))
		}
		cfg.ActionKeySalt = []byte(salt)
		cfg.saltExplicit = true
	} else if cfg.Token != "" {
		sum := sha256.Sum256([]byte(cfg.Token))
		cfg.ActionKeySalt = sum[:]
	}

	if len(problems) > 0 {
		return Config{}, errors.Join(problems...)
	}
	return cfg, nil
}

// Secrets returns the values a log redactor must remove: the token and, when
// set explicitly, the key of action_key.
func (c Config) Secrets() []string {
	out := []string{c.Token}
	if c.saltExplicit {
		out = append(out, string(c.ActionKeySalt))
	}
	return out
}

// Allowed reports whether a Telegram user id is on the allow-list.
func (c Config) Allowed(userID int64) bool {
	for _, id := range c.AllowedUserIDs {
		if id == userID {
			return true
		}
	}
	return false
}

// LogValue is what a Config looks like in a log line: the settings, the size
// of the allow-list, and neither the token, nor the salt, nor a user id.
func (c Config) LogValue() slog.Value {
	return slog.GroupValue(
		slog.Bool("token_set", c.Token != ""),
		slog.Int("allowed_users", len(c.AllowedUserIDs)),
		slog.String("gateway_url", printableURL(c.GatewayURL)),
		slog.Duration("poll_timeout", c.PollTimeout),
		slog.String("health_addr", c.HealthAddr),
		slog.Int("commands_per_min", c.CommandsPerMinute),
		slog.Bool("action_key_salt_explicit", c.saltExplicit),
	)
}

// String keeps the secrets out of %v and %+v.
func (c Config) String() string {
	return fmt.Sprintf("config{token_set=%t allowed_users=%d gateway_url=%s poll_timeout=%s health_addr=%s commands_per_min=%d}",
		c.Token != "", len(c.AllowedUserIDs), printableURL(c.GatewayURL), c.PollTimeout, c.HealthAddr, c.CommandsPerMinute)
}

// GoString keeps the secrets out of %#v.
func (c Config) GoString() string { return c.String() }

// printableURL is the gateway address as a log line or an error may show it:
// without the user info, the query and the fragment, which can carry a
// password or a key. A value that does not parse is withheld whole.
func printableURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return "<unparsable, withheld>"
	}
	u.User, u.RawQuery, u.ForceQuery, u.Fragment, u.RawFragment = nil, "", false, "", ""
	return u.String()
}

func tokenShaped(token string) bool {
	id, secret, ok := strings.Cut(token, ":")
	if !ok || id == "" || secret == "" {
		return false
	}
	_, err := strconv.ParseUint(id, 10, 64)
	return err == nil
}

// parseAllowList reads the entries of MV_TELEGRAM_ALLOWED_USER_IDS. A bad
// entry is reported by its position, not by its value: the neighbours of a typo
// are real user ids.
func parseAllowList(entries []string) ([]int64, error) {
	ids := make([]int64, 0, len(entries))
	for i, e := range entries {
		id, err := strconv.ParseInt(e, 10, 64)
		if err != nil || id <= 0 {
			return nil, fmt.Errorf("MV_TELEGRAM_ALLOWED_USER_IDS: entry %d is not a positive Telegram user id; value withheld", i+1)
		}
		ids = append(ids, id)
	}
	return ids, nil
}
