package main

// The llama-server double. It is used in two ways:
//
//   - in process, as the server a scenario of `health` or of the bench talks
//     to: the stand listens on 127.0.0.1:<free port> and answers the way the
//     configuration of the scenario says (the owner's server answers /health
//     with 404 and /v1/models with 200, which is the default here);
//   - as a program, when a scenario of `up` needs something to start: the stand
//     copies its own executable under a neutral name and hands the path to the
//     scripts as MV_LLM_BIN. The name is neutral on purpose — an antivirus
//     removed a double called llama-server.exe seconds after it was built
//     (review of T-437).
//
// In both roles every request is appended to requests.jsonl and every start of
// the program to launches.jsonl, so that a scenario can compare what the two
// implementations sent, not only what they printed.
//
// The program role owns no process but itself: it exits after its lifetime,
// or when the stand asks it to over HTTP with the token of this run. The stand
// never kills a process by number or by name.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	envRole     = "MVPARITY_ROLE"
	envDir      = "MVPARITY_DIR"
	envConfig   = "MVPARITY_CONFIG"
	envToken    = "MVPARITY_TOKEN"
	envLifetime = "MVPARITY_LIFETIME"
	envGPU      = "MVPARITY_GPU"
	roleDouble  = "llama-server-double"

	pathWhoami   = "/__parity/whoami"
	pathShutdown = "/__parity/shutdown"
	headerToken  = "X-Parity-Token"
)

// DoubleConfig is what a scenario tells the double. Zero values are the
// defaults of the comments.
type DoubleConfig struct {
	Health     int      `json:"health,omitempty"`      // /health status; 0: 404 in process (the owner's server), 200 as a program (llama.cpp)
	Models     []string `json:"models,omitempty"`      // ids of /v1/models; nil: the three test models in process, the alias or -m as a program
	NoModels   bool     `json:"no_models,omitempty"`   // /v1/models answers an empty list
	ModelsCode int      `json:"models_code,omitempty"` // 0: 200
	Props      string   `json:"props,omitempty"`       // build_info of /props; "": /props answers 404
	WarmCode   int      `json:"warm_code,omitempty"`   // status of a chat call with max_tokens 1; 0: 200 for a JSON body, 500 otherwise
	RequireKey string   `json:"require_key,omitempty"` // every endpoint but chat answers 401 unless Authorization is "Bearer <key>"
	Version    string   `json:"version,omitempty"`     // what --version prints; "": "build: 10878 (parity double)"

	PromptTokens     int     `json:"prompt_tokens,omitempty"`
	CachedTokens     int     `json:"cached_tokens,omitempty"`
	CompletionTokens int     `json:"completion_tokens,omitempty"`
	PredictedMS      float64 `json:"predicted_ms,omitempty"`
	// PromptMS: "all" sends timings.prompt_ms with every answer, "none" never,
	// "" every other prompt — chosen by a hash of the prompt text, so that both
	// implementations see the same answers whatever order they ask in.
	PromptMS string `json:"prompt_ms,omitempty"`
	Content  string `json:"content,omitempty"` // the message content of a chat answer
}

const defaultContent = `{"text":"Тёмный лес шумит, тропа уходит к реке.","events":[{"summary":"Волк ушёл в чащу."}]}`

// requestRecord is one line of requests.jsonl.
type requestRecord struct {
	Method    string `json:"method"`
	Path      string `json:"path"`
	Auth      string `json:"auth,omitempty"`
	Model     string `json:"model,omitempty"`
	MaxTokens int    `json:"max_tokens,omitempty"`
	BadJSON   bool   `json:"bad_json,omitempty"`
}

type double struct {
	cfg      DoubleConfig
	models   []string
	program  bool
	token    string
	logPath  string
	mu       sync.Mutex
	shutdown func()
}

func (d *double) record(rec requestRecord) {
	d.mu.Lock()
	defer d.mu.Unlock()
	line, _ := json.Marshal(rec)
	f, err := os.OpenFile(d.logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	_, _ = f.Write(append(line, '\n'))
}

func (d *double) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(io.LimitReader(r.Body, 8<<20))
	path := r.URL.Path

	switch path {
	case pathWhoami:
		exe, _ := os.Executable()
		writeJSON(w, http.StatusOK, map[string]any{"exe": exe, "pid": os.Getpid()})
		return
	case pathShutdown:
		if d.token == "" || r.Header.Get(headerToken) != d.token {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "wrong token"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "bye"})
		if d.shutdown != nil {
			go d.shutdown()
		}
		return
	}

	rec := requestRecord{Method: r.Method, Path: path, Auth: r.Header.Get("Authorization")}
	var chat struct {
		Model     string `json:"model"`
		MaxTokens int    `json:"max_tokens"`
		Messages  []struct {
			Content string `json:"content"`
		} `json:"messages"`
	}
	if path == "/v1/chat/completions" {
		if err := json.Unmarshal(body, &chat); err != nil {
			rec.BadJSON = true
		} else {
			rec.Model = chat.Model
			rec.MaxTokens = chat.MaxTokens
		}
	}
	d.record(rec)

	if d.cfg.RequireKey != "" && path != "/v1/chat/completions" &&
		rec.Auth != "Bearer "+d.cfg.RequireKey {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": map[string]any{"code": 401, "message": "Invalid API Key"}})
		return
	}

	switch path {
	case "/health":
		code := d.cfg.Health
		if code == 0 {
			code = http.StatusNotFound
			if d.program {
				code = http.StatusOK
			}
		}
		switch code {
		case http.StatusOK:
			writeJSON(w, code, map[string]string{"status": "ok"})
		case http.StatusServiceUnavailable:
			writeJSON(w, code, map[string]any{"error": map[string]any{"code": 503, "message": "Loading model"}})
		default:
			writeJSON(w, code, map[string]any{"error": map[string]any{"code": code, "message": "File Not Found"}})
		}
	case "/v1/models":
		code := d.cfg.ModelsCode
		if code == 0 {
			code = http.StatusOK
		}
		if code != http.StatusOK {
			writeJSON(w, code, map[string]any{"error": map[string]any{"code": code}})
			return
		}
		data := []map[string]any{}
		if !d.cfg.NoModels {
			for _, id := range d.models {
				data = append(data, map[string]any{"id": id, "object": "model", "owned_by": "llamacpp"})
			}
		}
		writeJSON(w, code, map[string]any{"object": "list", "data": data})
	case "/props":
		if d.cfg.Props == "" {
			writeJSON(w, http.StatusNotFound, map[string]any{"error": map[string]any{"code": 404}})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"build_info": d.cfg.Props, "total_slots": 1})
	case "/v1/chat/completions":
		d.chat(w, rec, chat.Messages)
	default:
		writeJSON(w, http.StatusNotFound, map[string]any{"error": map[string]any{"code": 404, "message": "File Not Found"}})
	}
}

func (d *double) chat(w http.ResponseWriter, rec requestRecord, messages []struct {
	Content string `json:"content"`
}) {
	if rec.BadJSON {
		// llama.cpp answers a body it cannot parse with 500: this is exactly how
		// the warm-up of T-402 failed on a model id with backslashes.
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": map[string]any{"code": 500, "message": "invalid JSON body"}})
		return
	}
	if rec.MaxTokens == 1 && d.cfg.WarmCode != 0 && d.cfg.WarmCode != http.StatusOK {
		writeJSON(w, d.cfg.WarmCode, map[string]any{"error": map[string]any{"code": d.cfg.WarmCode, "message": "warm-up refused by the double"}})
		return
	}
	prompt, cached, completion := d.cfg.PromptTokens, d.cfg.CachedTokens, d.cfg.CompletionTokens
	if prompt == 0 {
		prompt = 400
	}
	if completion == 0 {
		completion = 80
	}
	predicted := d.cfg.PredictedMS
	if predicted == 0 {
		predicted = 1200
	}
	timings := map[string]any{"predicted_ms": predicted}
	text := ""
	if len(messages) > 0 {
		text = messages[len(messages)-1].Content
	}
	h := fnv.New32a()
	_, _ = h.Write([]byte(text))
	switch d.cfg.PromptMS {
	case "all":
		timings["prompt_ms"] = 210.5
	case "none":
	default:
		if h.Sum32()%2 == 0 {
			timings["prompt_ms"] = 210.5
		}
	}
	content := d.cfg.Content
	if content == "" {
		content = defaultContent
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"id":      "chatcmpl-parity",
		"object":  "chat.completion",
		"model":   rec.Model,
		"choices": []map[string]any{{"index": 0, "finish_reason": "stop", "message": map[string]any{"role": "assistant", "content": content}}},
		"usage": map[string]any{
			"prompt_tokens": prompt, "completion_tokens": completion, "total_tokens": prompt + completion,
			"prompt_tokens_details": map[string]any{"cached_tokens": cached},
		},
		"timings": timings,
	})
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(v)
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_, _ = w.Write(buf.Bytes())
}

// serveInProcess starts the in-process double on ln and returns its stop.
func serveInProcess(ln net.Listener, cfg DoubleConfig, logPath string) func() {
	models := cfg.Models
	if models == nil {
		models = []string{"model-alpha", "model-beta", "model-gamma"}
	}
	d := &double{cfg: cfg, models: models, logPath: logPath}
	srv := &http.Server{Handler: d, ErrorLog: log.New(io.Discard, "", 0), ReadHeaderTimeout: 10 * time.Second}
	go func() { _ = srv.Serve(ln) }()
	return func() { _ = srv.Close() }
}

// runDouble is the program role: a llama-server as far as the scripts can tell.
func runDouble(args []string) int {
	dir := os.Getenv(envDir)
	var cfg DoubleConfig
	if raw := os.Getenv(envConfig); raw != "" {
		if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
			fmt.Fprintf(os.Stderr, "parity double: %s: %v\n", envConfig, err)
			return 3
		}
	}
	if dir != "" {
		appendJSONLine(filepath.Join(dir, "launches.jsonl"), map[string]any{"argv": args})
	}

	if len(args) == 1 && args[0] == "--version" {
		version := cfg.Version
		if version == "" {
			version = "build: 10878 (parity double)"
		}
		fmt.Println(version)
		return 0
	}

	host, port, alias, model := "127.0.0.1", "", "", ""
	router := false
	for i := 0; i < len(args); i++ {
		next := func() string {
			if i+1 < len(args) {
				i++
				return args[i]
			}
			return ""
		}
		switch args[i] {
		case "--host":
			host = next()
		case "--port":
			port = next()
		case "--alias":
			alias = next()
		case "-m":
			model = next()
		case "--models-dir":
			next()
			router = true
		}
	}
	if _, err := strconv.Atoi(port); err != nil {
		fmt.Fprintf(os.Stderr, "parity double: no usable --port in %q\n", args)
		return 3
	}
	models := cfg.Models
	if models == nil {
		switch {
		case alias != "":
			models = []string{alias}
		case model != "":
			// What llama.cpp reports without --alias: the path of the file.
			models = []string{model}
		case router:
			models = []string{"router-model-a", "router-model-b"}
		}
	}

	ln, err := net.Listen("tcp", net.JoinHostPort(strings.Trim(host, "[]"), port))
	if err != nil {
		fmt.Fprintf(os.Stderr, "parity double: listen: %v\n", err)
		return 3
	}
	done := make(chan struct{})
	var once sync.Once
	stop := func() { once.Do(func() { close(done) }) }
	d := &double{cfg: cfg, models: models, program: true, token: os.Getenv(envToken), logPath: filepath.Join(dir, "requests.jsonl"), shutdown: stop}
	if dir == "" {
		d.logPath = os.DevNull
	}
	srv := &http.Server{Handler: d, ErrorLog: log.New(io.Discard, "", 0), ReadHeaderTimeout: 10 * time.Second}
	go func() { _ = srv.Serve(ln) }()

	lifetime := 150 * time.Second
	if raw := os.Getenv(envLifetime); raw != "" {
		if s, err := strconv.Atoi(raw); err == nil && s > 0 {
			lifetime = time.Duration(s) * time.Second
		}
	}
	select {
	case <-done:
		// Let the answer to the shutdown request leave before the socket goes.
		time.Sleep(100 * time.Millisecond)
	case <-time.After(lifetime):
	}
	_ = srv.Close()
	return 0
}

func appendJSONLine(path string, v any) {
	line, err := json.Marshal(v)
	if err != nil {
		return
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	_, _ = f.Write(append(line, '\n'))
}

// gpuToolName is the name the scripts call for the VRAM report. The stand puts
// a copy of itself under that name first on PATH, so the report is the same on
// every machine, with or without a GPU, and the real tool is never called.
const gpuToolName = "nvidia-smi"

// runGPUTool answers the two queries the scripts make: llm-server asks for
// used and total memory with units, llm-bench for used memory without them.
//
// MVPARITY_GPU=fail makes it behave as nvidia-smi does when the driver does not
// answer: the complaint on stdout and exit 9.
func runGPUTool(args []string) int {
	if os.Getenv(envGPU) == "fail" {
		fmt.Println("NVIDIA-SMI has failed because it couldn't communicate with the NVIDIA driver.")
		return 9
	}
	joined := strings.Join(args, " ")
	switch {
	case strings.Contains(joined, "nounits"):
		fmt.Println("1234")
	default:
		fmt.Println("1234 MiB, 24564 MiB")
	}
	return 0
}
