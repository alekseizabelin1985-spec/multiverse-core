package agent

import (
	"os"
	"time"
)

// readFile helper for reading files
func readFile(path string) ([]byte, error) {
	return os.ReadFile(path)
}

// BlueprintFactory парсит блупринты
type BlueprintFactory interface {
	ParseBlueprint(data []byte) (*AgentBlueprint, error)
	ParseFile(path string) (*AgentBlueprint, error)
	ValidateBlueprint(bp *AgentBlueprint) error
}

// DefaultBlueprintFactory реализует BlueprintFactory
type DefaultBlueprintFactory struct {
	parser BlueprintParser
}

// NewDefaultBlueprintFactory создает новый factory
func NewDefaultBlueprintFactory() *DefaultBlueprintFactory {
	return &DefaultBlueprintFactory{
		parser: NewYAMLParser(),
	}
}

// ParseBlueprint парсит блупринт
func (f *DefaultBlueprintFactory) ParseBlueprint(data []byte) (*AgentBlueprint, error) {
	return f.parser.ParseYAML(data)
}

// ParseFile парсит файл
func (f *DefaultBlueprintFactory) ParseFile(path string) (*AgentBlueprint, error) {
	return f.parser.ParseFile(path)
}

// ValidateBlueprint валидирует
func (f *DefaultBlueprintFactory) ValidateBlueprint(bp *AgentBlueprint) error {
	return f.parser.Validate(bp)
}

// TTLManager управляет временем жизни агентов
type TTLManager struct {
	// CheckInterval интервал проверки
	CheckInterval time.Duration
	
	// DefaultTTL время жизни по умолчанию
	DefaultTTL time.Duration
	
	// mu для синхронизации
	mu sync.RWMutex
	
	// agentTTLs хранит TTL для агентов
	agentTTLs map[string]time.Time
	
	// cleanupFunc функция для очистки
	cleanupFunc func(agentID string) error
}

// NewTTLManager создает новый TTL менеджер
func NewTTLManager(checkInterval, defaultTTL time.Duration) *TTLManager {
	return &TTLManager{
		CheckInterval: checkInterval,
		DefaultTTL:    defaultTTL,
		agentTTLs:     make(map[string]time.Time),
	}
}

// SetTTL устанавливает TTL для агента
func (tm *TTLManager) SetTTL(agentID string, ttl time.Time) {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	tm.agentTTLs[agentID] = ttl
}

// GetTTL получает TTL для агента
func (tm *TTLManager) GetTTL(agentID string) (time.Time, bool) {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	
	ttl, exists := tm.agentTTLs[agentID]
	return ttl, exists
}

// RemoveTTL удаляет TTL
func (tm *TTLManager) RemoveTTL(agentID string) {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	delete(tm.agentTTLs, agentID)
}

// Expired проверяет, истек ли TTL
func (tm *TTLManager) Expired(agentID string) bool {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	
	ttl, exists := tm.agentTTLs[agentID]
	if !exists {
		return false // Нет TTL, не истекает
	}
	
	return time.Now().After(ttl)
}

// GetAllExpired возвращает все истекшие агенты
func (tm *TTLManager) GetAllExpired() []string {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	
	var expired []string
	for agentID, ttl := range tm.agentTTLs {
		if time.Now().After(ttl) {
			expired = append(expired, agentID)
		}
	}
	
	return expired
}

// SetCleanupFunc устанавливает функцию для очистки
func (tm *TTLManager) SetCleanupFunc(fn func(agentID string) error) {
	tm.cleanupFunc = fn
}

// CleanupExpired удаляет истекшие агенты
func (tm *TTLManager) CleanupExpired() ([]string, error) {
	expired := tm.GetAllExpired()
	
	for _, agentID := range expired {
		if tm.cleanupFunc != nil {
			if err := tm.cleanupFunc(agentID); err != nil {
				return expired, err
			}
		}
		tm.RemoveTTL(agentID)
	}
	
	return expired, nil
}

// StartAutoCleanup запускает автоочистку
func (tm *TTLManager) StartAutoCleanup(ctx context.Context) {
	ticker := time.NewTicker(tm.CheckInterval)
	
	go func() {
		defer ticker.Stop()
		
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				_, err := tm.CleanupExpired()
				if err != nil {
					// Логирование ошибки
					// TODO: добавить логгер
				}
			}
		}
	}()
}

// Stats возвращает статистику TTL
func (tm *TTLManager) Stats() map[string]interface{} {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	
	return map[string]interface{}{
		"total_agents":   len(tm.agentTTLs),
		"check_interval": tm.CheckInterval.String(),
		"default_ttl":    tm.DefaultTTL.String(),
	}
}
