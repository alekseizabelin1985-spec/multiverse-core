package agent

import (
	"context"
	"testing"
	"time"
)

func TestNewRouter(t *testing.T) {
	router := NewRouter(nil)
	
	if router.blueprints == nil {
		t.Error("Expected blueprints to be initialized")
	}
	
	if router.agents == nil {
		t.Error("Expected agents to be initialized")
	}
}

func TestRouterStats(t *testing.T) {
	router := NewRouter(nil)
	
	stats := router.Stats()
	
	if stats["blueprints_count"] != 0 {
		t.Error("Expected blueprints_count to be 0")
	}
	
	if stats["agents_count"] != 0 {
		t.Error("Expected agents_count to be 0")
	}
}

func TestNewWorkerPool(t *testing.T) {
	pool := NewWorkerPool(5)
	
	if pool.workers != 5 {
		t.Errorf("Expected 5 workers, got %d", pool.workers)
	}
	
	if cap(pool.jobChan) != 50 {
		t.Errorf("Expected jobChan capacity of 50, got %d", cap(pool.jobChan))
	}
}

func TestWorkerPoolStatus(t *testing.T) {
	pool := NewWorkerPool(3)
	
	status := pool.Status()
	
	if status.WorkersCount != 3 {
		t.Errorf("Expected 3 workers, got %d", status.WorkersCount)
	}
	
	if status.TotalProcessed != 0 {
		t.Error("Expected 0 processed jobs")
	}
	
	if status.TotalErrors != 0 {
		t.Error("Expected 0 errors")
	}
}

func TestNewTTLManager(t *testing.T) {
	checkInterval := 1 * time.Minute
	defaultTTL := 1 * time.Hour
	
	tm := NewTTLManager(checkInterval, defaultTTL)
	
	if tm.CheckInterval != checkInterval {
		t.Errorf("Expected checkInterval %v, got %v", checkInterval, tm.CheckInterval)
	}
	
	if tm.DefaultTTL != defaultTTL {
		t.Errorf("Expected defaultTTL %v, got %v", defaultTTL, tm.DefaultTTL)
	}
}

func TestTTLManagerSetAndGet(t *testing.T) {
	tm := NewTTLManager(1*time.Minute, 1*time.Hour)
	
	agentID := "test-agent"
	expireAt := time.Now().Add(1 * time.Hour)
	
	tm.SetTTL(agentID, expireAt)
	
	retrieved, exists := tm.GetTTL(agentID)
	if !exists {
		t.Error("Expected TTL to exist")
	}
	
	if !retrieved.Equal(expireAt) {
		t.Errorf("Expected %v, got %v", expireAt, retrieved)
	}
}

func TestTTLManagerExpired(t *testing.T) {
	tm := NewTTLManager(1*time.Minute, 1*time.Hour)
	
	agentID := "test-agent"
	
	// Set TTL to 1 hour
	tm.SetTTL(agentID, time.Now().Add(1*time.Hour))
	
	// Should not be expired yet
	if tm.Expired(agentID) {
		t.Error("Expected agent to not be expired")
	}
	
	// Set TTL to past
	tm.SetTTL(agentID, time.Now().Add(-1*time.Hour))
	
	// Should be expired
	if !tm.Expired(agentID) {
		t.Error("Expected agent to be expired")
	}
}

func TestTTLManagerGetAllExpired(t *testing.T) {
	tm := NewTTLManager(1*time.Minute, 1*time.Hour)
	
	tm.SetTTL("agent-1", time.Now().Add(-1*time.Hour))
	tm.SetTTL("agent-2", time.Now().Add(1*time.Hour))
	tm.SetTTL("agent-3", time.Now().Add(-2*time.Hour))
	
	expired := tm.GetAllExpired()
	
	if len(expired) != 2 {
		t.Errorf("Expected 2 expired agents, got %d", len(expired))
	}
}

func TestDefaultBlueprintFactory(t *testing.T) {
	factory := NewDefaultBlueprintFactory()
	
	if factory.parser == nil {
		t.Error("Expected parser to be initialized")
	}
}

func TestBlueprintTrigger(t *testing.T) {
	tr := BlueprintTrigger{
		Type:        "event",
		EventName:   "player.entered_region",
	}
	
	if tr.Type != "event" {
		t.Error("Expected Type to be 'event'")
	}
	
	if tr.EventName != "player.entered_region" {
		t.Error("Expected EventName to be 'player.entered_region'")
	}
}

func TestLODLevelString(t *testing.T) {
	tests := []struct {
		lod      LODLevel
		expected string
	}{
		{LODDisabled, "disabled"},
		{LODRuleOnly, "rule-only"},
		{LODBasic, "basic"},
		{LODFull, "full"},
		{LODLevel(99), "unknown"},
	}
	
	for _, tt := range tests {
		if tt.lod.String() != tt.expected {
			t.Errorf("Expected %s, got %s", tt.expected, tt.lod.String())
		}
	}
}

func TestAgentLevelString(t *testing.T) {
	tests := []struct {
		level  AgentLevel
		expected string
	}{
		{LevelGlobal, "global"},
		{LevelDomain, "domain"},
		{LevelTask, "task"},
		{LevelObject, "object"},
		{LevelMonitor, "monitor"},
		{AgentLevel(99), "unknown"},
	}
	
	for _, tt := range tests {
		if tt.level.String() != tt.expected {
			t.Errorf("Expected %s, got %s", tt.expected, tt.level.String())
		}
	}
}

func TestAgentLifecycleStateString(t *testing.T) {
	tests := []struct {
		state  AgentLifecycleState
		expected string
	}{
		{LifecycleInitializing, "initializing"},
		{LifecycleRunning, "running"},
		{LifecyclePaused, "paused"},
		{LifecycleFinished, "finished"},
		{AgentLifecycleState(99), "unknown"},
	}
	
	for _, tt := range tests {
		if tt.state.String() != tt.expected {
			t.Errorf("Expected %s, got %s", tt.expected, tt.state.String())
		}
	}
}

func TestWorkerPoolStatistics(t *testing.T) {
	pool := NewWorkerPool(5)
	
	stats := pool.Statistics()
	
	if stats["workers"] != float64(5) {
		t.Error("Expected workers to be 5")
	}
}

func TestTTLManagerStats(t *testing.T) {
	tm := NewTTLManager(1*time.Minute, 1*time.Hour)
	
	stats := tm.Stats()
	
	if stats["check_interval"] != "1m0s" {
		t.Error("Expected check_interval to be 1m0s")
	}
	
	if stats["default_ttl"] != "1h0m0s" {
		t.Error("Expected default_ttl to be 1h0m0s")
	}
}

// Benchmark for Router.MatchEvents
func BenchmarkRouterMatchEvents(b *testing.B) {
	router := NewRouter(nil)
	
	// Add some blueprints
	for i := 0; i < 100; i++ {
		router.RegisterBlueprint(&AgentBlueprint{
			Name:    "test-blueprint",
			Version: "1.0",
			Trigger: BlueprintTrigger{
				Type:      "event",
				EventName: "player.action",
			},
		})
	}
	
	event := Event{
		Type: "player.action",
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		router.MatchEvents(event)
	}
}

// Benchmark for WorkerPool
func BenchmarkWorkerPoolSubmit(b *testing.B) {
	pool := NewWorkerPool(10)
	
	agent := &MockAgent{}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pool.SubmitAsync(Job{
			Agent: agent,
			Event: Event{Type: "test"},
		})
	}
}

// MockAgent для тестов
type MockAgent struct{}

func (m *MockAgent) ID() string { return "mock-agent" }
func (m *MockAgent) Type() string { return "mock" }
func (m *MockAgent) Level() AgentLevel { return LevelDomain }
func (m *MockAgent) State() AgentLifecycleState { return LifecycleRunning }
func (m *MockAgent) Context() *AgentContext { return nil }
func (m *MockAgent) Tick(ctx context.Context, event Event) (Action, error) { return Action{}, nil }
func (m *MockAgent) HandleEvent(ctx context.Context, event Event) error { return nil }
func (m *MockAgent) Shutdown(ctx context.Context) error { return nil }
func (m *MockAgent) Pause(ctx context.Context) error { return nil }
func (m *MockAgent) Resume(ctx context.Context) error { return nil }
func (m *MockAgent) Memory() MemoryStore { return nil }
func (m *MockAgent) Tools() ToolRegistry { return nil }
