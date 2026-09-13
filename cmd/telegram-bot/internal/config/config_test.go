package config_test

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"testing"
	"time"

	"multiverse-core.io/cmd/telegram-bot/internal/config"
	"multiverse-core.io/shared/env"
)

const (
	token = "7012345678:AAHdqTcvCH1vGWJxfSeofSAs0K5PALDsawQ"
	salt  = "a-salt-of-twenty-chars"
)

func source(m map[string]string) env.Source {
	base := map[string]string{"MV_TELEGRAM_BOT_TOKEN": token}
	for k, v := range m {
		base[k] = v
	}
	return env.MapSource(base)
}

func TestLoadReadsTheManifestDefaults(t *testing.T) {
	cfg, err := config.Load(source(nil))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Token != token || cfg.GatewayURL != "http://127.0.0.1:8088" || cfg.PollTimeout != 25*time.Second ||
		cfg.HealthAddr != ":8089" || cfg.CommandsPerMinute != 20 {
		t.Fatalf("defaults: %+v", cfg)
	}
	if len(cfg.AllowedUserIDs) != 0 || cfg.Allowed(1) {
		t.Fatal("the default allow-list must be empty and admit nobody (SEC-06 fail-closed)")
	}
	want := sha256.Sum256([]byte(token))
	if !bytes.Equal(cfg.ActionKeySalt, want[:]) {
		t.Fatal("without MV_TELEGRAM_ACTION_KEY_SALT the key of action_key is SHA-256 of the token")
	}
	if got := cfg.Secrets(); len(got) != 1 || got[0] != token {
		t.Fatalf("a derived salt is not a secret of its own: %d secrets", len(got))
	}
}

func TestLoadReadsEveryVariable(t *testing.T) {
	cfg, err := config.Load(source(map[string]string{
		"MV_TELEGRAM_ALLOWED_USER_IDS": " 111, 222 ,,333",
		"MV_TELEGRAM_GATEWAY_URL":      "http://gateway:8088/",
		"MV_TELEGRAM_POLL_TIMEOUT_S":   "10",
		"MV_TELEGRAM_HEALTH_ADDR":      "127.0.0.1:9000",
		"MV_TELEGRAM_COMMANDS_PER_MIN": "5",
		"MV_TELEGRAM_ACTION_KEY_SALT":  salt,
	}))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if fmt.Sprint(cfg.AllowedUserIDs) != "[111 222 333]" || !cfg.Allowed(222) || cfg.Allowed(444) {
		t.Fatalf("allow-list: %v", cfg.AllowedUserIDs)
	}
	if cfg.GatewayURL != "http://gateway:8088" || cfg.PollTimeout != 10*time.Second || cfg.HealthAddr != "127.0.0.1:9000" ||
		cfg.CommandsPerMinute != 5 || string(cfg.ActionKeySalt) != salt {
		t.Fatalf("values: %+v", cfg)
	}
	if got := cfg.Secrets(); len(got) != 2 || got[1] != salt {
		t.Fatal("an explicit salt is a secret the redactor must know")
	}
}

func TestLoadRefusesAMissingToken(t *testing.T) {
	_, err := config.Load(env.MapSource(map[string]string{}))
	if !errors.Is(err, config.ErrTokenMissing) {
		t.Fatalf("Load without a token = %v, want ErrTokenMissing", err)
	}
}

func TestLoadErrorsWithholdSecretsAndUserIDs(t *testing.T) {
	_, err := config.Load(env.MapSource(map[string]string{
		"MV_TELEGRAM_BOT_TOKEN":        "not-a-token-AAHdqTcvCH1vGWJxfSeofSAs",
		"MV_TELEGRAM_ALLOWED_USER_IDS": "987654321,98765x321",
		"MV_TELEGRAM_ACTION_KEY_SALT":  "shortsalt",
		"MV_TELEGRAM_GATEWAY_URL":      "gateway:8088",
		"MV_TELEGRAM_POLL_TIMEOUT_S":   "0",
		"MV_TELEGRAM_COMMANDS_PER_MIN": "0",
	}))
	if err == nil {
		t.Fatal("Load accepted a broken configuration")
	}
	msg := err.Error()
	for _, leaked := range []string{"AAHdqTcvCH1vGWJxfSeofSAs", "987654321", "98765x321", "shortsalt"} {
		if strings.Contains(msg, leaked) {
			t.Fatalf("error text carries %q: %s", leaked, msg)
		}
	}
	for _, reported := range []string{"MV_TELEGRAM_BOT_TOKEN", "entry 2", "MV_TELEGRAM_ACTION_KEY_SALT", "MV_TELEGRAM_GATEWAY_URL", "MV_TELEGRAM_POLL_TIMEOUT_S", "MV_TELEGRAM_COMMANDS_PER_MIN"} {
		if !strings.Contains(msg, reported) {
			t.Fatalf("every problem is reported at once, %q is missing: %s", reported, msg)
		}
	}
}

func TestLoadRefusesNonNumericKinds(t *testing.T) {
	for _, name := range []string{"MV_TELEGRAM_POLL_TIMEOUT_S", "MV_TELEGRAM_COMMANDS_PER_MIN"} {
		if _, err := config.Load(source(map[string]string{name: "many"})); err == nil || !strings.Contains(err.Error(), name) {
			t.Fatalf("%s=many: err = %v", name, err)
		}
	}
	if _, err := config.Load(source(map[string]string{"MV_TELEGRAM_ALLOWED_USER_IDS": "-5"})); err == nil {
		t.Fatal("a negative user id must be refused")
	}
	if _, err := config.Load(source(map[string]string{"MV_TELEGRAM_ALLOWED_USER_IDS": "111,0"})); err == nil || !strings.Contains(err.Error(), "entry 2") {
		t.Fatalf("user id 0 must be refused by its position: %v", err)
	}
	if cfg, err := config.Load(source(map[string]string{"MV_TELEGRAM_ALLOWED_USER_IDS": "1"})); err != nil || !cfg.Allowed(1) {
		t.Fatalf("user id 1 is the smallest valid one: %v", err)
	}
}

// TestLoadCountsTheSaltInCharacters pins the edge of MinSaltLen, in characters
// as the manifest says: 16 are enough whatever their bytes, 15 are not.
func TestLoadCountsTheSaltInCharacters(t *testing.T) {
	cases := []struct {
		salt string
		ok   bool
	}{
		{strings.Repeat("s", config.MinSaltLen), true},
		{strings.Repeat("s", config.MinSaltLen-1), false},
		{strings.Repeat("ж", config.MinSaltLen), true},
		{strings.Repeat("ж", config.MinSaltLen-1), false},
	}
	for _, c := range cases {
		_, err := config.Load(source(map[string]string{"MV_TELEGRAM_ACTION_KEY_SALT": c.salt}))
		if (err == nil) != c.ok {
			t.Errorf("salt of %d characters (%d bytes): err = %v, want accepted = %t", len([]rune(c.salt)), len(c.salt), err, c.ok)
		}
	}
}

// TestTheGatewayURLIsPrintedWithoutCredentials: a user info, a query or a
// fragment of MV_TELEGRAM_GATEWAY_URL can carry a password or a key.
func TestTheGatewayURLIsPrintedWithoutCredentials(t *testing.T) {
	_, err := config.Load(source(map[string]string{"MV_TELEGRAM_GATEWAY_URL": "ftp://admin:hunter2secret@gateway:8088/?key=k3y-value#frag"}))
	if err == nil || !strings.Contains(err.Error(), "gateway:8088") {
		t.Fatalf("a non-http URL must be refused and still named by its host: %v", err)
	}
	for _, leaked := range []string{"admin", "hunter2secret", "k3y-value", "frag"} {
		if strings.Contains(err.Error(), leaked) {
			t.Fatalf("error text carries %q: %v", leaked, err)
		}
	}
	if _, err := config.Load(source(map[string]string{"MV_TELEGRAM_GATEWAY_URL": "http://admin:hunter2secret@gate\x7fway"})); err == nil || strings.Contains(err.Error(), "hunter2secret") {
		t.Fatalf("an unparsable URL must be refused without its value: %v", err)
	}

	cfg, err := config.Load(source(map[string]string{"MV_TELEGRAM_GATEWAY_URL": "https://bot:hunter2secret@gateway:8088/base?key=k3y-value"}))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	var buf bytes.Buffer
	slog.New(slog.NewJSONHandler(&buf, nil)).Info("config", slog.Any("cfg", cfg))
	for _, out := range []string{cfg.String(), buf.String()} {
		if strings.Contains(out, "hunter2secret") || strings.Contains(out, "k3y-value") || !strings.Contains(out, "https://gateway:8088/base") {
			t.Fatalf("printed config: %s", out)
		}
	}
	if cfg.GatewayURL != "https://bot:hunter2secret@gateway:8088/base?key=k3y-value" {
		t.Fatal("the configured URL itself is kept whole: only its printed form is cut")
	}
}

func TestConfigPrintsWithoutSecrets(t *testing.T) {
	cfg, err := config.Load(source(map[string]string{
		"MV_TELEGRAM_ALLOWED_USER_IDS": "987654321",
		"MV_TELEGRAM_ACTION_KEY_SALT":  salt,
	}))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	var buf bytes.Buffer
	slog.New(slog.NewJSONHandler(&buf, nil)).Info("config", slog.Any("cfg", cfg))
	for _, out := range []string{fmt.Sprint(cfg), fmt.Sprintf("%+v", cfg), fmt.Sprintf("%#v", cfg), fmt.Sprintf("%v", &cfg), buf.String()} {
		for _, leaked := range []string{"AAHdqTcvCH1vGWJxfSeofSAs0K5PALDsawQ", "987654321", salt} {
			if strings.Contains(out, leaked) {
				t.Fatalf("printed config carries %q: %s", leaked, out)
			}
		}
	}
	if !strings.Contains(buf.String(), `"allowed_users":1`) {
		t.Fatalf("the log form should still say how many users are allowed: %s", buf.String())
	}
}
