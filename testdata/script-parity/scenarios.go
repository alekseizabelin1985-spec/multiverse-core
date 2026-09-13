package main

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// The scenarios. The first block is the matrix of T-404 (review #1, 24 inputs;
// review #2 and iteration 3, the inputs that found M-1…M-3, Mi-9…Mi-12), the
// second the start and stop of the server (T-404 iteration 3, T-437), the third
// the bench (T-434, and the rounding of the table from its review, N-1).
//
// Inputs deliberately NOT here:
//   - a backslash in MV_LLM_URL: bash turns it into a slash on the way to pwsh,
//     so the two halves never see the same value (T-404 review #2);
//   - an address outside this machine that is actually probed (the cloud gate
//     with a 200): it needs a name that resolves to loopback through DNS, and
//     the stand does not leave the machine. The classification of such an
//     address is covered through `down`, which does not probe;
//   - a pid file naming a live process that is not ours: under Git Bash the
//     number of a native Windows process is not one kill -0 can see, so the two
//     halves disagree for a reason of the environment, not of the scripts.

func allScenarios() []Scenario {
	var all []Scenario
	all = append(all, healthScenarios()...)
	all = append(all, upDownScenarios()...)
	all = append(all, benchScenarios()...)
	return all
}

func health(expect Expect) []Step { return []Step{{Action: "health", Expect: expect}} }

// ownerLike is the server of the owner's stand: /health 404, /v1/models 200.
func ownerLike() *DoubleConfig { return &DoubleConfig{} }

func healthScenarios() []Scenario {
	url := "http://127.0.0.1:{PORT}"
	return []Scenario{
		{ID: "H01", Title: "health: /health 404 falls back to /v1/models (the owner's server)", Script: scriptServer,
			Env: map[string]string{"MV_LLM_URL": url}, Server: ownerLike(),
			Steps: health(Expect{Exit: 0,
				Contains: []string{"llm: probing http://127.0.0.1:<PORT> (from MV_LLM_URL=http://127.0.0.1:<PORT>)",
					"/v1/models = 200 ok (this server has no /health", "llm: models = model-alpha, model-beta, model-gamma",
					"MV_LLM_BIN is not set — this runtime was not started from here"},
				Requests: []string{"GET /health", "GET /v1/models"}})},
		{ID: "H02", Title: "health: /health answers 200", Script: scriptServer,
			Env: map[string]string{"MV_LLM_URL": url}, Server: &DoubleConfig{Health: 200},
			Steps: health(Expect{Exit: 0, Contains: []string{"/health = 200 ok"}, Requests: []string{"GET /health", "GET /v1/models"}})},
		{ID: "H03", Title: "health: host.docker.internal is probed at 127.0.0.1", Script: scriptServer,
			Env: map[string]string{"MV_LLM_URL": "http://host.docker.internal:{PORT}"}, Server: ownerLike(),
			Steps: health(Expect{Exit: 0, Contains: []string{"probing http://127.0.0.1:<PORT> (from MV_LLM_URL=http://host.docker.internal:<PORT>)"}})},
		{ID: "H04", Title: "health: a trailing /v1 is cut off", Script: scriptServer,
			Env: map[string]string{"MV_LLM_URL": url + "/v1"}, Server: ownerLike(),
			Steps: health(Expect{Exit: 0, Contains: []string{"ends with /v1; the variable holds the base address"}, Requests: []string{"GET /health", "GET /v1/models"}})},
		{ID: "H05", Title: "health: a trailing /v1/ is cut off", Script: scriptServer,
			Env: map[string]string{"MV_LLM_URL": url + "/v1/"}, Server: ownerLike(),
			Steps: health(Expect{Exit: 0, Requests: []string{"GET /health", "GET /v1/models"}})},
		{ID: "H06", Title: "health: //v1 is cut off", Script: scriptServer,
			Env: map[string]string{"MV_LLM_URL": url + "//v1"}, Server: ownerLike(),
			Steps: health(Expect{Exit: 0, Requests: []string{"GET /health", "GET /v1/models"}})},
		{ID: "H07", Title: "health: /V1 is a path, not the suffix", Script: scriptServer,
			Env: map[string]string{"MV_LLM_URL": url + "/V1"}, Server: ownerLike(),
			Steps: health(Expect{Exit: 1, Requests: []string{"GET /V1/health", "GET /V1/v1/models"}})},
		{ID: "H08", Title: "health: a path of several segments is kept", Script: scriptServer,
			Env: map[string]string{"MV_LLM_URL": url + "/api/openai"}, Server: ownerLike(),
			Steps: health(Expect{Exit: 1, Requests: []string{"GET /api/openai/health", "GET /api/openai/v1/models"}})},
		{ID: "H09", Title: "health: HTTP in capitals", Script: scriptServer,
			Env: map[string]string{"MV_LLM_URL": "HTTP://127.0.0.1:{PORT}"}, Server: ownerLike(),
			Steps: health(Expect{Exit: 0})},
		{ID: "H10", Title: "health: MV_LLM_URL empty (review C-1)", Script: scriptServer,
			Env:   map[string]string{"MV_LLM_URL": ""},
			Steps: health(Expect{Exit: 1, Stderr: []string{"llm: MV_LLM_URL has no value — the platform has no LLM address"}})},
		{ID: "H11", Title: "health: MV_LLM_URL not set", Script: scriptServer,
			Steps: health(Expect{Exit: 1, Stderr: []string{"llm: MV_LLM_URL has no value"}})},
		{ID: "H12", Title: "health: MV_LLM_URL of spaces", Script: scriptServer,
			Env:   map[string]string{"MV_LLM_URL": "   "},
			Steps: health(Expect{Exit: 1, Stderr: []string{"has no value"}})},
		{ID: "H13", Title: "health: no scheme (review Mi-11)", Script: scriptServer,
			Env:   map[string]string{"MV_LLM_URL": "127.0.0.1:{PORT}"},
			Steps: health(Expect{Exit: 1, Stderr: []string{"has no scheme; write it as http://host:port"}})},
		{ID: "H14", Title: "health: ftp scheme", Script: scriptServer,
			Env:   map[string]string{"MV_LLM_URL": "ftp://127.0.0.1:{PORT}"},
			Steps: health(Expect{Exit: 1, Stderr: []string{"uses scheme 'ftp'"}})},
		{ID: "H15", Title: "health: user information in the address is refused and not printed (M-4, Mi-4)", Script: scriptServer,
			Env:   map[string]string{"MV_LLM_URL": "http://user:s3cr3t@127.0.0.1:{PORT}"},
			Steps: health(Expect{Exit: 1, Stderr: []string{"carries user information in front of the host"}, Absent: []string{"s3cr3t"}})},
		{ID: "H16", Title: "health: an empty user information", Script: scriptServer,
			Env:   map[string]string{"MV_LLM_URL": "http://@127.0.0.1:{PORT}"},
			Steps: health(Expect{Exit: 1, Stderr: []string{"carries user information"}})},
		{ID: "H17", Title: "health: [::1] is ours and nobody listens (M-3)", Script: scriptServer,
			Env:   map[string]string{"MV_LLM_URL": "http://[::1]:{DEAD}"},
			Steps: health(Expect{Exit: 1, Contains: []string{"unreachable — the process is not running"}, Absent: []string{"points outside this machine"}})},
		{ID: "H18", Title: "health: a local address without a port (Mi-12)", Script: scriptServer,
			Env:   map[string]string{"MV_LLM_URL": "http://[::1]"},
			Steps: health(Expect{Exit: 1, Stderr: []string{"has no port; the local runtime would bind 80"}})},
		{ID: "H19", Title: "health: a bare IPv6 literal", Script: scriptServer,
			Env:   map[string]string{"MV_LLM_URL": "http://::1:{DEAD}"},
			Steps: health(Expect{Exit: 1, Stderr: []string{"looks like a bare IPv6 literal"}})},
		{ID: "H20", Title: "health: characters after the IPv6 literal (Mi-9)", Script: scriptServer,
			Env:   map[string]string{"MV_LLM_URL": "http://[::1]x"},
			Steps: health(Expect{Exit: 1, Stderr: []string{"has trailing characters after the IPv6 literal"}})},
		{ID: "H21", Title: "health: an unclosed IPv6 literal", Script: scriptServer,
			Env:   map[string]string{"MV_LLM_URL": "http://[::1:{DEAD}"},
			Steps: health(Expect{Exit: 1, Stderr: []string{"has an unclosed IPv6 literal"}})},
		{ID: "H22", Title: "health: 127.0.0.1 without a port", Script: scriptServer,
			Env:   map[string]string{"MV_LLM_URL": "http://127.0.0.1"},
			Steps: health(Expect{Exit: 1, Stderr: []string{"has no port"}})},
		{ID: "H23", Title: "health: port 0", Script: scriptServer,
			Env:   map[string]string{"MV_LLM_URL": "http://127.0.0.1:0"},
			Steps: health(Expect{Exit: 1, Stderr: []string{"has port 0, which is outside 1-65535"}})},
		{ID: "H24", Title: "health: port 99999", Script: scriptServer,
			Env:   map[string]string{"MV_LLM_URL": "http://127.0.0.1:99999"},
			Steps: health(Expect{Exit: 1, Stderr: []string{"outside 1-65535"}})},
		{ID: "H25", Title: "health: a port that is not a number", Script: scriptServer,
			Env:   map[string]string{"MV_LLM_URL": "http://127.0.0.1:abc"},
			Steps: health(Expect{Exit: 1, Stderr: []string{"has a non-numeric port 'abc'"}})},
		{ID: "H26", Title: "health: no host before the path", Script: scriptServer,
			Env:   map[string]string{"MV_LLM_URL": "http:///v1"},
			Steps: health(Expect{Exit: 1, Stderr: []string{"has no host"}})},
		{ID: "H27", Title: "health: no host before the port", Script: scriptServer,
			Env:   map[string]string{"MV_LLM_URL": "http://:{DEAD}"},
			Steps: health(Expect{Exit: 1, Stderr: []string{"has no host"}})},
		{ID: "H28", Title: "health: a query string", Script: scriptServer,
			Env:   map[string]string{"MV_LLM_URL": "http://127.0.0.1:{PORT}/?x=1"},
			Steps: health(Expect{Exit: 1, Stderr: []string{"carries a query or a fragment"}, Absent: []string{"x=1"}})},
		{ID: "H29", Title: "health: a fragment", Script: scriptServer,
			Env:   map[string]string{"MV_LLM_URL": "http://127.0.0.1:{PORT}#top"},
			Steps: health(Expect{Exit: 1, Stderr: []string{"carries a query or a fragment"}})},
		{ID: "H30", Title: "health: a soft hyphen after /v1 (review #2 M-1)", Script: scriptServer,
			Env: map[string]string{"MV_LLM_URL": url + "/v1\u00ad"}, Server: ownerLike(),
			Steps: health(Expect{Exit: 1, Stderr: []string{"carries a character outside printable ASCII"}, Requests: []string{}})},
		{ID: "H31", Title: "health: a zero width space after /v1 (review #2 M-1)", Script: scriptServer,
			Env: map[string]string{"MV_LLM_URL": url + "/v1\u200b"}, Server: ownerLike(),
			Steps: health(Expect{Exit: 1, Stderr: []string{"outside printable ASCII"}, Requests: []string{}})},
		{ID: "H32", Title: "health: a soft hyphen inside the scheme (review #2 M-1)", Script: scriptServer,
			Env:   map[string]string{"MV_LLM_URL": "http\u00ad://127.0.0.1:{PORT}"},
			Steps: health(Expect{Exit: 1, Stderr: []string{"outside printable ASCII"}})},
		{ID: "H33", Title: "health: a no-break space at the end", Script: scriptServer,
			Env: map[string]string{"MV_LLM_URL": url + "\u00a0"}, Server: ownerLike(),
			Steps: health(Expect{Exit: 1, Stderr: []string{"outside printable ASCII"}})},
		{ID: "H34", Title: "health: TLS to a plain HTTP port is a fault, not 'not running' (review #2 M-2)", Script: scriptServer,
			Env: map[string]string{"MV_LLM_URL": "https://127.0.0.1:{PORT}"}, Server: ownerLike(),
			Steps: health(Expect{Exit: 1, Contains: []string{"llm: the request itself failed — this is not \"the process is not running\""}})},
		{ID: "H35", Title: "health: MV_LLM_PROVIDER empty means openai_compat (Mi-10)", Script: scriptServer,
			Env: map[string]string{"MV_LLM_URL": url, "MV_LLM_PROVIDER": ""}, Server: ownerLike(),
			Steps: health(Expect{Exit: 0, Contains: []string{"MV_LLM_BIN is not set"}})},
		{ID: "H36", Title: "health: a provider outside the manifest (review #2 Mi-11)", Script: scriptServer,
			Env:   map[string]string{"MV_LLM_URL": url, "MV_LLM_PROVIDER": "zzz"},
			Steps: health(Expect{Exit: 1, Stderr: []string{"llm: MV_LLM_PROVIDER=zzz is not one of openai_compat, ollama, anthropic, recorded, fake"}})},
		{ID: "H37", Title: "health: anthropic has no address here", Script: scriptServer,
			Env:   map[string]string{"MV_LLM_PROVIDER": "anthropic"},
			Steps: health(Expect{Exit: 0, Contains: []string{"is the vendor cloud API — not started from here"}})},
		{ID: "H38", Title: "health: recorded answers inside the process", Script: scriptServer,
			Env:   map[string]string{"MV_LLM_PROVIDER": "recorded"},
			Steps: health(Expect{Exit: 0, Contains: []string{"MV_LLM_PROVIDER=recorded answers inside the process"}})},
		{ID: "H39", Title: "health: fake answers inside the process", Script: scriptServer,
			Env:   map[string]string{"MV_LLM_PROVIDER": "fake"},
			Steps: health(Expect{Exit: 0, Contains: []string{"MV_LLM_PROVIDER=fake answers inside the process"}})},
		{ID: "H40", Title: "health: ollama is probed at MV_OLLAMA_URL, without pin and VRAM", Script: scriptServer,
			Env: map[string]string{"MV_LLM_PROVIDER": "ollama", "MV_OLLAMA_URL": url, "MV_LLM_URL": "http://127.0.0.1:{DEAD}"}, Server: ownerLike(),
			Steps: health(Expect{Exit: 0, Contains: []string{"(from MV_OLLAMA_URL=http://127.0.0.1:<PORT>)"}, Absent: []string{"llm: VRAM", "MV_LLM_BIN is not set"}})},
		{ID: "H41", Title: "health: ollama without MV_OLLAMA_URL", Script: scriptServer,
			Env:   map[string]string{"MV_LLM_PROVIDER": "ollama", "MV_LLM_URL": url},
			Steps: health(Expect{Exit: 1, Stderr: []string{"llm: MV_LLM_PROVIDER=ollama but MV_OLLAMA_URL is empty"}})},
		{ID: "H42", Title: "health: MV_LLM_PORT is retired and said so (Mi-1)", Script: scriptServer,
			Env: map[string]string{"MV_LLM_URL": url, "MV_LLM_PORT": "1234"}, Server: ownerLike(),
			Steps: health(Expect{Exit: 0, Contains: []string{"llm: MV_LLM_PORT=1234 is retired and ignored — the port comes from MV_LLM_URL"}})},
		{ID: "H43", Title: "health: a key with a space, quotes, a backslash, # and =; arrives verbatim (Mi-3)", Script: scriptServer,
			Env:    map[string]string{"MV_LLM_URL": url, "MV_LLM_API_KEY": `sp ace "quo" back\slash #h =;x`},
			Server: &DoubleConfig{RequireKey: `sp ace "quo" back\slash #h =;x`},
			Steps: health(Expect{Exit: 0, Contains: []string{"/v1/models = 200 ok"}, Auth: `Bearer sp ace "quo" back\slash #h =;x`,
				Absent: []string{`back\slash #h`}})},
		{ID: "H44", Title: "health: a key with CR LF is refused (review #2 M-3)", Script: scriptServer,
			Env: map[string]string{"MV_LLM_URL": url, "MV_LLM_API_KEY": "AB\r\nX-Injected: yes\r\nCD"}, Server: ownerLike(),
			Steps: health(Expect{Exit: 1, Stderr: []string{"llm: MV_LLM_API_KEY carries a control character"}, Requests: []string{}})},
		{ID: "H45", Title: "health: a key shorter than 8 characters is not redacted (Mi-10)", Script: scriptServer,
			Env: map[string]string{"MV_LLM_URL": url, "MV_LLM_API_KEY": "{PORT}"}, Server: ownerLike(),
			Steps: health(Expect{Exit: 0, Contains: []string{"probing http://127.0.0.1:<PORT>"}})},
		{ID: "H46", Title: "health: a key that appears in the output is redacted", Script: scriptServer,
			Env: map[string]string{"MV_LLM_URL": url, "MV_LLM_API_KEY": "127.0.0.1"}, Server: ownerLike(),
			Steps: health(Expect{Exit: 0, Contains: []string{"probing http://***:<PORT>"}})},
		{ID: "H47", Title: "health: 503 while the model loads", Script: scriptServer,
			Env: map[string]string{"MV_LLM_URL": url}, Server: &DoubleConfig{Health: 503},
			Steps: health(Expect{Exit: 1, Contains: []string{"/health = 503 loading (the model is still being read)"}})},
		{ID: "H48", Title: "health: 401 without a key", Script: scriptServer,
			Env: map[string]string{"MV_LLM_URL": url}, Server: &DoubleConfig{RequireKey: "the-right-key-0"},
			Steps: health(Expect{Exit: 1, Contains: []string{"= 401 — the endpoint needs a key; MV_LLM_API_KEY is empty"}})},
		{ID: "H49", Title: "health: 401 with a wrong key", Script: scriptServer,
			Env: map[string]string{"MV_LLM_URL": url, "MV_LLM_API_KEY": "the-wrong-key-1"}, Server: &DoubleConfig{RequireKey: "the-right-key-0"},
			Steps: health(Expect{Exit: 1, Contains: []string{"MV_LLM_API_KEY is set and rejected"}, Absent: []string{"the-wrong-key-1"}})},
		{ID: "H50", Title: "health: /health 500", Script: scriptServer,
			Env: map[string]string{"MV_LLM_URL": url}, Server: &DoubleConfig{Health: 500},
			Steps: health(Expect{Exit: 1, Contains: []string{"/health = 500"}})},
		{ID: "H51", Title: "health: /v1/models lists nothing", Script: scriptServer,
			Env: map[string]string{"MV_LLM_URL": url}, Server: &DoubleConfig{NoModels: true},
			Steps: health(Expect{Exit: 0, Contains: []string{"llm: models = <none reported>"}})},
		{ID: "H52", Title: "health: MV_LLM_BIN reports the pinned build", Script: scriptServer,
			Env: map[string]string{"MV_LLM_URL": url, "MV_LLM_BIN": "{DOUBLE}"}, Server: ownerLike(), Program: &DoubleConfig{},
			Steps: health(Expect{Exit: 0, Contains: []string{"llm: build b10878 matches the pin"}})},
		{ID: "H53", Title: "health: MV_LLM_BIN reports another build", Script: scriptServer,
			Env: map[string]string{"MV_LLM_URL": url, "MV_LLM_BIN": "{DOUBLE}"}, Server: ownerLike(), Program: &DoubleConfig{Version: "build: 10999 (parity double)"},
			Steps: health(Expect{Exit: 0, Contains: []string{"llm: WARNING build b10999 differs from the pin b10878 in build/versions.env"}})},
		{ID: "H54", Title: "health: scripts/lib is missing (review #2 Mi-3)", Script: scriptServer,
			Env: map[string]string{"MV_LLM_URL": url}, Setup: removeLib,
			Steps: health(Expect{Exit: 1, Stderr: []string{"/scripts/lib/", "is missing — it holds the only rule that turns MV_LLM_URL into an address"}})},
		{ID: "H55", Title: "health: nobody listens and MV_LLM_BIN is not set (review #2 Mi-1, Mi-2)", Script: scriptServer,
			Env: map[string]string{"MV_LLM_URL": "http://127.0.0.1:{DEAD}"},
			Steps: health(Expect{Exit: 1, Contains: []string{"unreachable — the process is not running; start it the way it was started before",
				"its GET /v1/models, which this run could not read"}})},
		{ID: "H57", Title: "health: MV_LLM_URL of a no-break space alone", Script: scriptServer,
			Env:   map[string]string{"MV_LLM_URL": "\u00a0"},
			Steps: health(Expect{Exit: 1, Stderr: []string{"outside printable ASCII"}})},
		{ID: "H58", Title: "health: an em space U+2003 after the port", Script: scriptServer,
			Env: map[string]string{"MV_LLM_URL": url + "\u2003"}, Server: ownerLike(),
			Steps: health(Expect{Exit: 1, Stderr: []string{"outside printable ASCII"}, Requests: []string{}})},
		{ID: "H59", Title: "health: a TAB and spaces around the address are trimmed", Script: scriptServer,
			Env: map[string]string{"MV_LLM_URL": "\t  " + url + "  \t"}, Server: ownerLike(),
			Steps: health(Expect{Exit: 0, Contains: []string{"(from MV_LLM_URL=http://127.0.0.1:<PORT>)"}})},
		{ID: "H60", Title: "health: nvidia-smi that fails is reported as it is, the health verdict does not change", Script: scriptServer,
			Env: map[string]string{"MV_LLM_URL": url, envGPU: "fail"}, Server: ownerLike(),
			Steps: health(Expect{Exit: 0, Contains: []string{"llm: VRAM NVIDIA-SMI has failed because it couldn't communicate with the NVIDIA driver."}})},
		{ID: "H56", Title: "health: nobody listens and MV_LLM_BIN is set", Script: scriptServer,
			Env: map[string]string{"MV_LLM_URL": "http://127.0.0.1:{DEAD}", "MV_LLM_BIN": "{DOUBLE}"}, Program: &DoubleConfig{},
			Steps: health(Expect{Exit: 1, Contains: []string{"unreachable — the process is not running (make llm-up)"}})},
		// T-450 review #2 Mi-R2-1: every refusal that prints the value cuts the
		// user information out of it, the ones that come before the check of the @
		// included. FAKEPW123 is a fake password; it must never reach the output.
		{ID: "H61", Title: "health: user information in an address with a query is not printed (T-450 review #2 Mi-R2-1)", Script: scriptServer,
			Env: map[string]string{"MV_LLM_URL": "http://user:FAKEPW123@127.0.0.1:{PORT}/?x=1"},
			Steps: health(Expect{Exit: 1, Stderr: []string{"MV_LLM_URL=http://…@127.0.0.1:<PORT>/… carries a query or a fragment"},
				Absent: []string{"FAKEPW123", "x=1"}})},
		{ID: "H62", Title: "health: user information in an address with the scheme ftp is not printed (T-450 review #2 Mi-R2-1)", Script: scriptServer,
			Env: map[string]string{"MV_LLM_URL": "ftp://user:FAKEPW123@127.0.0.1:{PORT}"},
			Steps: health(Expect{Exit: 1, Stderr: []string{"MV_LLM_URL=ftp://…@127.0.0.1:<PORT> uses scheme 'ftp'"},
				Absent: []string{"FAKEPW123"}})},
		{ID: "H63", Title: "health: user information in an address without a scheme is not printed (T-450 review #2 Mi-R2-1)", Script: scriptServer,
			Env: map[string]string{"MV_LLM_URL": "user:FAKEPW123@127.0.0.1:{PORT}"},
			Steps: health(Expect{Exit: 1, Stderr: []string{"MV_LLM_URL=…@127.0.0.1:<PORT> has no scheme"},
				Absent: []string{"FAKEPW123"}})},
		{ID: "H64", Title: "health: user information in an address with a character outside ASCII is not printed (T-450 review #2 Mi-R2-1)", Script: scriptServer,
			Env: map[string]string{"MV_LLM_URL": "http://user:FAKEPW123Ä@127.0.0.1:{PORT}"},
			Steps: health(Expect{Exit: 1, Stderr: []string{"MV_LLM_URL=http://…@127.0.0.1:<PORT> carries a character outside printable ASCII"},
				Absent: []string{"FAKEPW123"}})},
		{ID: "H65", Title: "health: a password with a bare ? is not printed in front of the cut (T-450 review #2 Mi-R2-1)", Script: scriptServer,
			Env: map[string]string{"MV_LLM_URL": "http://u:FAKEPW123?w@127.0.0.1:{PORT}"},
			Steps: health(Expect{Exit: 1, Stderr: []string{"MV_LLM_URL=http://… carries a query or a fragment"},
				Absent: []string{"FAKEPW123"}})},
		{ID: "H66", Title: "health: a password with a bare / is not printed by the sentence about the port (T-450 review #2 Mi-R2-1)", Script: scriptServer,
			Env: map[string]string{"MV_LLM_URL": "http://u:x/FAKEPW123@127.0.0.1:{PORT}"},
			Steps: health(Expect{Exit: 1, Stderr: []string{"MV_LLM_URL=http://…@127.0.0.1:<PORT> has a non-numeric port 'x'"},
				Absent: []string{"FAKEPW123"}})},
		// T-463, C-15 v1.5 "Печать значения": an @ after the first / leaves the
		// head of a password where the parser sees the host, and the value is
		// accepted by llm_parse_url and refused by the judge. Every sentence of
		// the judge prints such a value by its scheme alone and does not name
		// the host. One scenario per sentence the judge can reach; the sentence
		// of the other invalid kinds (malformed) is unreachable while
		// llm_parse_url refuses a malformed host first, and only the static
		// comparison of the messages holds it. fakepw is a fake password.
		{ID: "H67", Title: "health: an @ after the first /, a one-word host without a port: neither the value nor the host is printed (T-463)", Script: scriptServer,
			Env: map[string]string{"MV_LLM_URL": "http://fakepw/x@127.0.0.1:{PORT}"},
			Steps: health(Expect{Exit: 1,
				Stderr:     []string{"MV_LLM_URL=http://… has no port; a local address is written with the port its runtime listens on, and 80, the default of the scheme, is a port nobody chose (the rest of the value and its host are not printed"},
				AbsentFold: []string{"fakepw", "127.0.0.1", "/x@"}})},
		{ID: "H68", Title: "health: the same value in mixed case: the host is lower-cased by the parser and still not printed (T-463)", Script: scriptServer,
			Env: map[string]string{"MV_LLM_URL": "HTTP://FakePW/x@127.0.0.1:{PORT}"},
			Steps: health(Expect{Exit: 1,
				Stderr:     []string{"MV_LLM_URL=http://… has no port; a local address is written"},
				AbsentFold: []string{"fakepw", "127.0.0.1"}})},
		{ID: "H69", Title: "health: an @ after the first /, localhost without a port: the sentence of the startable kinds is masked too (T-463)", Script: scriptServer,
			Env: map[string]string{"MV_LLM_URL": "https://localhost/fakepw@127.0.0.1:{PORT}"},
			Steps: health(Expect{Exit: 1,
				Stderr:     []string{"MV_LLM_URL=https://… has no port; the local runtime would bind 443, the default of the scheme, and the probe would knock there — write the port you mean (the rest of the value and its host are not printed"},
				AbsentFold: []string{"fakepw", "127.0.0.1", "localhost"}})},
		// The sentence of the any-address advises "write 127.0.0.1" by itself, so
		// the address behind the @ here is another one.
		{ID: "H70", Title: "health: an @ after the first /, the any-address: the host is not named (T-463)", Script: scriptServer,
			Env: map[string]string{"MV_LLM_URL": "http://0.0.0.0:{PORT}/fakepw@10.0.0.5:{PORT}"},
			Steps: health(Expect{Exit: 1,
				Stderr:     []string{"MV_LLM_URL=http://… names the any-address: a server listens there, a client cannot call it — write 127.0.0.1 or the address of the host, with the port (the rest of the value and its host are not printed"},
				AbsentFold: []string{"fakepw", "0.0.0.0", "10.0.0.5"}})},
		{ID: "H71", Title: "health: without an @ the value and its host are printed as before (T-463)", Script: scriptServer,
			Env: map[string]string{"MV_LLM_URL": "http://fakehost/x"},
			Steps: health(Expect{Exit: 1,
				Stderr:     []string{"MV_LLM_URL=http://fakehost/x has no port; a local address is written with the port its runtime listens on, and 80, the default of the scheme, is a port nobody chose"},
				AbsentFold: []string{"are not printed"}})},
	}
}

// removeLib takes scripts/lib out of the tree of the run.
func removeLib(r *implRun) error {
	return os.RemoveAll(filepath.Join(r.tree, "scripts", "lib"))
}

func pidFile(content string) func(r *implRun) error {
	return func(r *implRun) error {
		return writeFile(filepath.Join(r.tree, "ops", "llm-server.pid"), content)
	}
}

// serverArgv is what both halves must hand to llama-server with the defaults of
// .env.example: the fixed arguments, then `between` (the slot save path, the
// models directory of router mode), then -m and --alias when model is set,
// then `after` (--no-webui, or the tools of --with-ui).
func serverArgv(model, alias string, between []string, after ...string) []string {
	argv := []string{"--host", "127.0.0.1", "--port", "<PORT>", "--ctx-size", "8192", "--parallel", "1",
		"--n-gpu-layers", "99", "--threads", "32", "--batch-size", "16000", "--flash-attn", "on",
		"--kv-offload", "--kv-unified", "--jinja", "--reasoning", "off", "--load-mode", "mmap+mlock"}
	argv = append(argv, between...)
	if model != "" {
		argv = append(argv, "-m", model, "--alias", alias)
	}
	return append(argv, after...)
}

func upDownScenarios() []Scenario {
	url := "http://127.0.0.1:{PORT}"
	upEnv := func(extra map[string]string) map[string]string {
		env := map[string]string{"MV_LLM_URL": url, "MV_LLM_BIN": "{DOUBLE}"}
		for k, v := range extra {
			env[k] = v
		}
		return env
	}
	down := Step{Action: "down", Expect: Expect{Exit: 0, Contains: []string{"llm: stopping llama-server (pid <PID>)", "llm: stopped; models and volumes are untouched"}, PortFree: true, NoFile: "ops/llm-server.pid"}}
	e := `D:\Models\unsloth\Qwen3.8\Qwen3.8-27B-UD-Q3_K_XL.gguf`

	return []Scenario{
		{ID: "U01", Title: "up/down: single mode, a Windows path, --alias is the file stem (T-437)", Script: scriptServer,
			Env: upEnv(map[string]string{"MV_LLM_MODEL_FILE": e}), Program: &DoubleConfig{},
			Steps: []Step{
				{Action: "up", ReadModels: true, Expect: Expect{Exit: 0,
					Contains: []string{"llm: starting <DOUBLE> on 127.0.0.1:<PORT> (single mode); the platform calls it at http://127.0.0.1:<PORT>",
						"llm: the model is reported as Qwen3.8-27B-UD-Q3_K_XL (--alias, the file stem that blueprints name)",
						"llm: http://127.0.0.1:<PORT>/health = 200 (pid <PID>, log <TREE>/ops/llm-server.log)", "llm: first call <MS> ms"},
					Argv:   serverArgv(e, "Qwen3.8-27B-UD-Q3_K_XL", nil, "--no-webui"),
					Models: []string{"Qwen3.8-27B-UD-Q3_K_XL"}}},
				down,
			}},
		{ID: "U02", Title: "up/down: .GGUF in capitals (T-437)", Script: scriptServer,
			Env: upEnv(map[string]string{"MV_LLM_MODEL_FILE": `D:\Models\Qwen3.6-35B-A3B-UD-Q3_K_XL.GGUF`}), Program: &DoubleConfig{},
			Steps: []Step{
				{Action: "up", ReadModels: true, Expect: Expect{Exit: 0, Models: []string{"Qwen3.6-35B-A3B-UD-Q3_K_XL"},
					Argv: serverArgv(`D:\Models\Qwen3.6-35B-A3B-UD-Q3_K_XL.GGUF`, "Qwen3.6-35B-A3B-UD-Q3_K_XL", nil, "--no-webui")}},
				down,
			}},
		{ID: "U03", Title: "up/down: dots in the name and no extension keep the name (T-437)", Script: scriptServer,
			Env: upEnv(map[string]string{"MV_LLM_MODEL_FILE": `D:\Models\Qwen3.8-27B`}), Program: &DoubleConfig{},
			Steps: []Step{
				{Action: "up", ReadModels: true, Expect: Expect{Exit: 0, Models: []string{"Qwen3.8-27B"},
					Argv: serverArgv(`D:\Models\Qwen3.8-27B`, "Qwen3.8-27B", nil, "--no-webui")}},
				down,
			}},
		{ID: "U04", Title: "up/down: router mode has no -m and no --alias (T-437)", Script: scriptServer,
			Env: upEnv(map[string]string{"MV_LLM_MODELS_DIR": `D:\Models\`, "MV_LLM_MODEL_FILE": e}), Program: &DoubleConfig{},
			Steps: []Step{
				{Action: "up", Router: true, Expect: Expect{Exit: 0, Absent: []string{"is reported as"},
					Argv: serverArgv("", "", []string{"--models-dir", `D:\Models\`, "--models-max", "2"}, "--no-webui")}},
				down,
			}},
		{ID: "U05", Title: "up: a model file that names no file is refused before the start (T-437)", Script: scriptServer,
			Env: upEnv(map[string]string{"MV_LLM_MODEL_FILE": `D:\Models\`}), Program: &DoubleConfig{},
			Steps: []Step{{Action: "up", Expect: Expect{Exit: 1, NoLaunch: true,
				Stderr: []string{`llm: MV_LLM_MODEL_FILE=D:\Models\ names no file, so the server would have no model id to report`}}}}},
		{ID: "U06", Title: "up/down: spaces in -m and in the alias stay one argument each (T-437 backlog)", Script: scriptServer,
			Env: upEnv(map[string]string{"MV_LLM_MODEL_FILE": `D:\My Models\Qwen 3.8.gguf`}), Program: &DoubleConfig{},
			Steps: []Step{
				{Action: "up", ReadModels: true, Expect: Expect{Exit: 0, Models: []string{"Qwen 3.8"},
					Argv: serverArgv(`D:\My Models\Qwen 3.8.gguf`, "Qwen 3.8", nil, "--no-webui")}},
				down,
			}},
		{ID: "U07", Title: "up/down: a models directory with a space and a trailing backslash (T-437 backlog)", Script: scriptServer,
			Env: upEnv(map[string]string{"MV_LLM_MODELS_DIR": `D:\My Models\`}), Program: &DoubleConfig{},
			Steps: []Step{
				{Action: "up", Router: true, Expect: Expect{Exit: 0,
					Argv: serverArgv("", "", []string{"--models-dir", `D:\My Models\`, "--models-max", "2"}, "--no-webui")}},
				down,
			}},
		{ID: "U08", Title: "up/down: a slot save path with spaces, a quote and a trailing backslash (T-437 backlog)", Script: scriptServer,
			Env: upEnv(map[string]string{"MV_LLM_MODEL_FILE": e, "MV_LLM_SLOT_SAVE_PATH": `D:\LLM "slots"\`}), Program: &DoubleConfig{},
			Steps: []Step{
				{Action: "up", Expect: Expect{Exit: 0,
					Argv: serverArgv(e, "Qwen3.8-27B-UD-Q3_K_XL", []string{"--slot-save-path", `D:\LLM "slots"\`}, "--no-webui")}},
				down,
			}},
		{ID: "U09", Title: "up: MV_LLM_HOST=0.0.0.0 is refused (SEC-15, Mi-12)", Script: scriptServer,
			Env: upEnv(map[string]string{"MV_LLM_MODEL_FILE": e, "MV_LLM_HOST": "0.0.0.0"}), Program: &DoubleConfig{},
			Steps: []Step{{Action: "up", Expect: Expect{Exit: 1, NoLaunch: true, Stderr: []string{"llm: MV_LLM_HOST=0.0.0.0 is not loopback"}}}}},
		{ID: "U10", Title: "up/down: blank MV_LLM_NUM_CTX and MV_LLM_SLOTS mean the defaults (Mi-9)", Script: scriptServer,
			Env: upEnv(map[string]string{"MV_LLM_MODEL_FILE": e, "MV_LLM_NUM_CTX": "", "MV_LLM_SLOTS": " "}), Program: &DoubleConfig{},
			Steps: []Step{
				{Action: "up", Expect: Expect{Exit: 0, Argv: serverArgv(e, "Qwen3.8-27B-UD-Q3_K_XL", nil, "--no-webui")}},
				down,
			}},
		{ID: "U11", Title: "up: somebody else answers on the address and no pid file (M-6)", Script: scriptServer,
			Env: upEnv(map[string]string{"MV_LLM_MODEL_FILE": e}), Server: ownerLike(), Program: &DoubleConfig{},
			Steps: []Step{{Action: "up", Expect: Expect{Exit: 1, NoLaunch: true,
				Stderr: []string{"/v1/models already answers 200 and ops/llm-server.pid records no process of ours — refusing to start a second server"}}}}},
		{ID: "U12", Title: "up twice: the second call is a no-op", Script: scriptServer,
			Env: upEnv(map[string]string{"MV_LLM_MODEL_FILE": e}), Program: &DoubleConfig{},
			Steps: []Step{
				{Action: "up", Expect: Expect{Exit: 0}},
				{Action: "up", Expect: Expect{Exit: 0, NoLaunch: true, Contains: []string{"llm: already running (pid <PID>) on http://127.0.0.1:<PORT> — nothing to do"}}},
				down,
			}},
		{ID: "U13", Title: "up/down: --with-ui adds the tools and the MCP proxy", Script: scriptServer,
			Env: upEnv(map[string]string{"MV_LLM_MODEL_FILE": e}), Program: &DoubleConfig{},
			Steps: []Step{
				{Action: "up", WithUI: true, Expect: Expect{Exit: 0, Contains: []string{"llm: WARNING web UI and MCP proxy are on (--with-ui); do not publish the port"},
					Argv: serverArgv(e, "Qwen3.8-27B-UD-Q3_K_XL", nil, "--tools", "all", "--ui-mcp-proxy")}},
				down,
			}},
		{ID: "U14", Title: "up: MV_LLM_BIN not set", Script: scriptServer,
			Env:   map[string]string{"MV_LLM_URL": url, "MV_LLM_MODEL_FILE": e},
			Steps: []Step{{Action: "up", Expect: Expect{Exit: 1, Stderr: []string{"llm: MV_LLM_BIN is not set (see .env.example, §4.2)"}}}}},
		{ID: "U15", Title: "up: MV_LLM_BIN points at nothing", Script: scriptServer,
			Env:   map[string]string{"MV_LLM_URL": url, "MV_LLM_MODEL_FILE": e, "MV_LLM_BIN": "{RUN}/no-such-server"},
			Steps: []Step{{Action: "up", Expect: Expect{Exit: 1, Stderr: []string{"no-such-server, which is not executable"}}}}},
		{ID: "U16", Title: "up/down: the warm-up call answers 404", Script: scriptServer,
			Env: upEnv(map[string]string{"MV_LLM_MODEL_FILE": e}), Program: &DoubleConfig{WarmCode: 404},
			Steps: []Step{
				{Action: "up", Expect: Expect{Exit: 0, Contains: []string{"llm: warm-up call answered 404; the server is up, check the blueprint model name"}}},
				down,
			}},
		{ID: "U17", Title: "up: router mode without MV_LLM_MODELS_DIR", Script: scriptServer,
			Env:   upEnv(nil),
			Steps: []Step{{Action: "up", Router: true, Expect: Expect{Exit: 1, NoLaunch: true, Stderr: []string{"llm: ROUTER mode needs MV_LLM_MODELS_DIR"}}}}},
		{ID: "U18", Title: "up: single mode without MV_LLM_MODEL_FILE", Script: scriptServer,
			Env:   upEnv(nil),
			Steps: []Step{{Action: "up", Expect: Expect{Exit: 1, NoLaunch: true, Stderr: []string{"llm: MV_LLM_MODEL_FILE is not set (single mode needs -m)"}}}}},
		// T-463 review #1 Ma-1: the refusal of U11 printed the probe, and the
		// probe carries the host and the path of the value. An @ after the first
		// / may end a password with a bare / in it (C-15 v1.5), so the refusal
		// names the variable instead. The double answers 404 on the unknown
		// path, /health included, which is still "somebody answers". fakepw is a
		// fake password.
		{ID: "U19", Title: "up: somebody else answers, the value holds an @ after the first /: the refusal prints neither the value nor its host (T-463)", Script: scriptServer,
			Env: upEnv(map[string]string{"MV_LLM_URL": "http://127.0.0.1:{PORT}/fakepw@x", "MV_LLM_MODEL_FILE": e}), Server: ownerLike(), Program: &DoubleConfig{},
			Steps: []Step{{Action: "up", Expect: Expect{Exit: 1, NoLaunch: true,
				Stderr: []string{"llm: the address of MV_LLM_URL (http://…) already answers 404 at /health and ops/llm-server.pid records no process of ours — refusing to start a second server on the same address",
					"(the rest of the value and its host are not printed: the value holds an @"},
				AbsentFold: []string{"fakepw", "@x"}}}}},
		{ID: "D01", Title: "down: no pid file", Script: scriptServer,
			Env:   map[string]string{"MV_LLM_URL": url},
			Steps: []Step{{Action: "down", Expect: Expect{Exit: 0, Contains: []string{"llm: no live server in ops/llm-server.pid — nothing to stop"}}}}},
		{ID: "D02", Title: "down: a pid file with junk (Mi-8)", Script: scriptServer,
			Env: map[string]string{"MV_LLM_URL": url}, Setup: pidFile("not-a-pid\n"),
			Steps: []Step{{Action: "down", Expect: Expect{Exit: 0, NoFile: "ops/llm-server.pid",
				Contains: []string{"llm: ops/llm-server.pid does not contain a process id — ignoring it", "nothing to stop"}}}}},
		{ID: "D03", Title: "down: a stale pid", Script: scriptServer,
			Env: map[string]string{"MV_LLM_URL": url}, Setup: pidFile("999999\nbin=/nowhere/llama-server\nstarted=2026-01-01T00:00:00Z\n"),
			Steps: []Step{{Action: "down", Expect: Expect{Exit: 0, NoFile: "ops/llm-server.pid", Contains: []string{"nothing to stop"}}}}},
		{ID: "D04", Title: "down: an address of the LAN is not ours, and nothing is probed", Script: scriptServer,
			Env:   map[string]string{"MV_LLM_URL": "http://10.1.2.3:{DEAD}"},
			Steps: []Step{{Action: "down", Expect: Expect{Exit: 0, Contains: []string{"llm: MV_LLM_URL=http://10.1.2.3:<DEAD> is not an address of this machine — provider openai_compat is not started or stopped from here"}}}}},
		{ID: "D05", Title: "down: a cloud address is not ours, and nothing is probed", Script: scriptServer,
			Env:   map[string]string{"MV_LLM_URL": "https://api.example.org/v1"},
			Steps: []Step{{Action: "down", Expect: Expect{Exit: 0, Contains: []string{"is not an address of this machine"}}}}},
		{ID: "D06", Title: "down: MV_LLM_URL empty", Script: scriptServer,
			Env:   map[string]string{"MV_LLM_URL": ""},
			Steps: []Step{{Action: "down", Expect: Expect{Exit: 1, Stderr: []string{"has no value"}}}}},
		// T-450: the table testdata/llm/local-endpoints.tsv is run through both
		// modules by T01/T02; these two carry a change of the rule into the script.
		{ID: "D07", Title: "down: the any-address 0.0.0.0 is a configuration error, no longer this machine (T-450)", Script: scriptServer,
			Env: map[string]string{"MV_LLM_URL": "http://0.0.0.0:{DEAD}"},
			Steps: []Step{{Action: "down", Expect: Expect{Exit: 1,
				Stderr: []string{"llm: MV_LLM_URL=http://0.0.0.0:<DEAD> names 0.0.0.0, the any-address: a server listens there, a client cannot call it"},
				Absent: []string{"nothing to stop"}}}}},
		{ID: "D08", Title: "down: an address of the LAN without a port is refused like a loopback one (T-450)", Script: scriptServer,
			Env: map[string]string{"MV_LLM_URL": "http://192.168.1.10"},
			Steps: []Step{{Action: "down", Expect: Expect{Exit: 1,
				Stderr: []string{"llm: MV_LLM_URL=http://192.168.1.10 has no port; a local address is written with the port its runtime listens on, and 80, the default of the scheme, is a port nobody chose"},
				Absent: []string{"is not an address of this machine"}}}}},
	}
}

func benchScenarios() []Scenario {
	url := "http://127.0.0.1:{PORT}"
	model := "Qwen3.8-27B-UD-Q3_K_XL"
	idPath := `D:\Models\fake\Qwen"x\model.gguf`
	base := func() *DoubleConfig {
		return &DoubleConfig{Models: []string{model}, Props: "b10878-parity", PromptTokens: 400, CachedTokens: 174, CompletionTokens: 80, PredictedMS: 1200}
	}
	bench := func(args BenchArgs, expect Expect) []Step {
		return []Step{{Action: "bench", Bench: &args, Expect: expect}}
	}
	out := "{RUN}/out"

	return []Scenario{
		{ID: "B01", Title: "bench: the matrix of the repository, /props, a 200 warm-up; the cache share 174/400 is 0.43 in both (T-434 N-1)", Script: scriptBench,
			Env: map[string]string{"MV_LLM_URL": url}, Server: base(),
			Steps: bench(BenchArgs{Configs: "E", Limit: 2, OutDir: out}, Expect{Exit: 0,
				Contains: []string{"bench: E build b10878(props)"},
				Bench: &BenchExpect{Rows: 3, Reports: 1, Cache: "0.43",
					Columns:  map[string]string{"tps": "66.7", "llamacpp_build": "b10878(props)", "first_call_ms": "<set>", "valid_json_ratio": "1.0", "lang_pass_ratio": "1.0", "cached_tok": "348", "prompt_tok": "800"},
					Meta:     map[string]string{"matrix": "ops/metrics/bench-matrix.json", "prompts": "testdata/bench/prompts.jsonl", "url": "http://127.0.0.1:<PORT>", "probe_url": "http://127.0.0.1:<PORT>"},
					MetaFile: map[string]string{"matrix_sha256": "{TREE}/ops/metrics/bench-matrix.json", "prompts_sha256": "{TREE}/testdata/bench/prompts.jsonl"},
					PromptMS: "mixed"}})},
		{ID: "B02", Title: "bench: tps 1333/20 s lands on a decimal midpoint: 66.7 in both (T-405)", Script: scriptBench,
			Env:    map[string]string{"MV_LLM_URL": url},
			Server: &DoubleConfig{Models: []string{model}, Props: "b10878-parity", PromptTokens: 800, CachedTokens: 348, CompletionTokens: 1333, PredictedMS: 20000},
			Steps: bench(BenchArgs{Configs: "E", Limit: 1, OutDir: out}, Expect{Exit: 0,
				Bench: &BenchExpect{Rows: 3, Reports: 1, Cache: "0.43", Columns: map[string]string{"tps": "66.7"}}})},
		{ID: "B03", Title: "bench: warm-up 500, no /props, no timings.prompt_ms: first_call_ms empty, the pin (T-434)", Script: scriptBench,
			Env:    map[string]string{"MV_LLM_URL": url},
			Server: &DoubleConfig{Models: []string{model}, WarmCode: 500, PromptMS: "none"},
			Steps: bench(BenchArgs{Configs: "E", Limit: 2, OutDir: out}, Expect{Exit: 0,
				Contains: []string{"bench: warm-up call answered 500 — first_call_ms left empty, the cold call was not measured", "bench: E build b10878(pin)"},
				Bench: &BenchExpect{Rows: 3, Reports: 1, Columns: map[string]string{"first_call_ms": "", "llamacpp_build": "b10878(pin)"},
					Meta: map[string]string{"first_call_ms": "null"}, PromptMS: "null"}})},
		{ID: "B04", Title: "bench: a matrix outside the tree with a path id that has backslashes and a quote (T-434)", Script: scriptBench,
			Env:    map[string]string{"MV_LLM_URL": url + "/v1"},
			Server: &DoubleConfig{Models: []string{idPath}, Props: "b10878-parity"},
			Setup:  matrixCopy(idPath),
			Steps: bench(BenchArgs{Configs: "E", Limit: 1, Matrix: "{RUN}/matrix-copy.json"}, Expect{Exit: 0,
				Bench: &BenchExpect{Rows: 3, Reports: 1, Columns: map[string]string{"first_call_ms": "<set>", "model": idPath},
					Meta:     map[string]string{"matrix": "matrix-copy.json", "url": "http://127.0.0.1:<PORT>/v1", "probe_url": "http://127.0.0.1:<PORT>"},
					MetaFile: map[string]string{"matrix_sha256": "{RUN}/matrix-copy.json"}}})},
		{ID: "B05", Title: "bench: Latin in the narrative fails the language verdict; ratios round alike", Script: scriptBench,
			Env: map[string]string{"MV_LLM_URL": url},
			Server: &DoubleConfig{Models: []string{model}, Props: "b10878-parity", PromptTokens: 777, CachedTokens: 111, CompletionTokens: 333, PredictedMS: 4321.5,
				Content: `{"text":"Лес шумит, wolf рядом.","events":[{"summary":"Ветер, rain и туман."}]}`},
			Steps: bench(BenchArgs{Configs: "E", Limit: 2, OutDir: out}, Expect{Exit: 0,
				Bench: &BenchExpect{Rows: 3, Reports: 1, Cache: "0.14", Columns: map[string]string{"lang_pass_ratio": "0.0", "latin_ratio": "<set>"}}})},
		{ID: "B06", Title: "bench: the model is not served — every phase skipped, nothing measured", Script: scriptBench,
			Env: map[string]string{"MV_LLM_URL": url}, Server: &DoubleConfig{Models: []string{"some-other-model"}},
			Steps: bench(BenchArgs{Configs: "E", Limit: 1, OutDir: out}, Expect{Exit: 2,
				Contains: []string{"bench: model Qwen3.8-27B-UD-Q3_K_XL is not in http://127.0.0.1:<PORT>/v1/models — phase tick skipped (server mode single?)",
					"bench: nothing was measured — every phase was skipped or failed; see the messages above"},
				Bench: &BenchExpect{Rows: 0, Reports: 0}})},
		{ID: "B07", Title: "bench: nobody listens", Script: scriptBench,
			Env: map[string]string{"MV_LLM_URL": "http://127.0.0.1:{DEAD}"},
			Steps: bench(BenchArgs{Configs: "E", Limit: 1, OutDir: out}, Expect{Exit: 2,
				Contains: []string{"bench: no answer from http://127.0.0.1:<DEAD> — the LLM process is not running. Start it: make llm-up (config E expects MV_LLM_URL=http://127.0.0.1:<DEAD>)"},
				Bench:    &BenchExpect{Rows: 0, Reports: 0}})},
		{ID: "B08", Title: "bench: a configuration the matrix does not have", Script: scriptBench,
			Env: map[string]string{"MV_LLM_URL": url}, Server: base(),
			Steps: bench(BenchArgs{Configs: "ZZ", Limit: 1, OutDir: out}, Expect{Exit: 2,
				Contains: []string{"bench: configuration ZZ is not in <TREE>/ops/metrics/bench-matrix.json (have: "},
				Bench:    &BenchExpect{Rows: 0, Reports: 0}})},
		{ID: "B09", Title: "bench: scripts/lib is missing", Script: scriptBench,
			Env: map[string]string{"MV_LLM_URL": url}, Setup: removeLib,
			Steps: bench(BenchArgs{Configs: "E", Limit: 1, OutDir: out}, Expect{Exit: 2,
				Contains: []string{"is missing — it holds the only rule that turns MV_LLM_URL into an address"},
				Bench:    &BenchExpect{Rows: 0, Reports: 0}})},
		{ID: "B10", Title: "bench: the report and the CSV are named relative to the repository when they are inside it", Script: scriptBench,
			Env: map[string]string{"MV_LLM_URL": url}, Server: base(),
			Steps: bench(BenchArgs{Configs: "E", Limit: 1}, Expect{Exit: 0,
				Contains: []string{"bench: ops/metrics/bench-E-<STAMP>.json", "bench: ops/metrics/bench-<STAMP>.csv (3 rows)"},
				Bench:    &BenchExpect{Rows: 3, Reports: 1}})},
		{ID: "B11", Title: "bench: nvidia-smi that fails (driver not answering) leaves vram_used_mb empty, the run goes on", Script: scriptBench,
			Env: map[string]string{"MV_LLM_URL": url, envGPU: "fail"}, Server: base(),
			Steps: bench(BenchArgs{Configs: "E", Limit: 1, OutDir: out}, Expect{Exit: 0,
				// No Absent on the raw output here: it carries the time stamp of the
				// report and the random number of the scratch directory, and "1234"
				// matched a run started at 12:34 (review #2 of T-405, Mi-2). The empty
				// column, the empty meta field and M16 hold the point of the scenario.
				Bench: &BenchExpect{Rows: 3, Reports: 1, Columns: map[string]string{"vram_used_mb": ""},
					Meta: map[string]string{"vram_used_mb": ""}}})},

		// The two reports of known defects. They run every check and every
		// expectation like the rest; only the items listed here may differ, and
		// all of them must (known.go).
		{ID: "K01", Title: "bench, known defect: five message lines go to stderr in bash and to stdout in PowerShell", Script: scriptBench,
			Env: map[string]string{"MV_LLM_URL": url}, Server: base(), ParityOnly: true,
			Reports: map[string][]string{"K01": {"prompts", "build", "report", "csv", "one run"}},
			Steps: bench(BenchArgs{Configs: "E", Limit: 1, OutDir: out}, Expect{Exit: 0,
				Bench: &BenchExpect{Rows: 3, Reports: 1, Columns: map[string]string{"vram_used_mb": "1234"}}})},
		{ID: "K02", Title: "bench, known defect: seven fields of the JSON report have different types in the two halves", Script: scriptBench,
			Env: map[string]string{"MV_LLM_URL": url}, Server: base(), ParityOnly: true,
			Reports: map[string][]string{"K02": {"meta.num_ctx", "meta.n_per_cell", "meta.repeats", "meta.runs_required",
				"meta.first_call_ms", "requests[].http", "cells[].vram_used_mb"}},
			Steps: bench(BenchArgs{Configs: "E", Limit: 1, OutDir: out}, Expect{Exit: 0,
				Bench: &BenchExpect{Rows: 3, Reports: 1, Meta: map[string]string{"vram_used_mb": "1234"}}})},
	}
}

// matrixCopy writes a copy of the matrix outside the tree, with every model of
// configuration E replaced by id — the shape llama-server reports without
// --alias.
func matrixCopy(id string) func(r *implRun) error {
	return func(r *implRun) error {
		raw, err := os.ReadFile(filepath.Join(r.tree, "ops", "metrics", "bench-matrix.json"))
		if err != nil {
			return err
		}
		var doc map[string]any
		if err := json.Unmarshal(raw, &doc); err != nil {
			return err
		}
		configs, _ := doc["configs"].(map[string]any)
		e, _ := configs["E"].(map[string]any)
		if e == nil {
			return os.ErrNotExist
		}
		e["tick"], e["narrative"] = id, id
		out, err := json.MarshalIndent(doc, "", "  ")
		if err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(r.run, "matrix-copy.json"), out, 0o644)
	}
}
