// internal/redis/client.go
package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// Client Redis клиент для hot cache
type Client struct {
	conn    Conn
	timeout time.Duration
}

// Conn интерфейс Redis соединения
type Conn interface {
	Get(ctx context.Context, key string) ([]byte, error)
	Set(ctx context.Context, key string, value []byte, expiration time.Duration) error
	Delete(ctx context.Context, key string) error
	Exists(ctx context.Context, keys ...string) (int64, error)
	Expire(ctx context.Context, key string, expiration time.Duration) error
	Ping(ctx context.Context) error
	Close() error
}

// Config конфигурация Redis клиента
type Config struct {
	Host            string
	Port            int
	Password        string
	DB              int
	PoolSize        int
	ConnTimeout     time.Duration
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	IdleTimeout     time.Duration
	MaxRetries      int
	RetryMinBackoff time.Duration
	RetryMaxBackoff time.Duration
}

// DefaultConfig возвращает конфигурацию по умолчанию
func DefaultConfig() Config {
	return Config{
		Host:            "redis",
		Port:            6379,
		Password:        "",
		DB:              0,
		PoolSize:        100,
		ConnTimeout:     5 * time.Second,
		ReadTimeout:     3 * time.Second,
		WriteTimeout:    3 * time.Second,
		IdleTimeout:     5 * time.Minute,
		MaxRetries:      3,
		RetryMinBackoff: 100 * time.Millisecond,
		RetryMaxBackoff: 1 * time.Second,
	}
}

// NewClient создает новый Redis клиент
func NewClient(cfg Config) (*Client, error) {
	// В production здесь будет реальный Redis клиент (например, go-redis)
	// Для примера используем заглушку
	conn := &stubConn{
		data: make(map[string][]byte),
	}

	return &Client{
		conn:    conn,
		timeout: cfg.ConnTimeout,
	}, nil
}

// Get получает значение из Redis
func (c *Client) Get(ctx context.Context, key string) ([]byte, error) {
	ctx, cancel := c.timeoutContext(ctx)
	defer cancel()

	return c.conn.Get(ctx, key)
}

// Set устанавливает значение в Redis
func (c *Client) Set(ctx context.Context, key string, value []byte, expiration time.Duration) error {
	ctx, cancel := c.timeoutContext(ctx)
	defer cancel()

	return c.conn.Set(ctx, key, value, expiration)
}

// Delete удаляет значение из Redis
func (c *Client) Delete(ctx context.Context, key string) error {
	ctx, cancel := c.timeoutContext(ctx)
	defer cancel()

	return c.conn.Delete(ctx, key)
}

// Exists проверяет существует ли ключ
func (c *Client) Exists(ctx context.Context, keys ...string) (int64, error) {
	ctx, cancel := c.timeoutContext(ctx)
	defer cancel()

	return c.conn.Exists(ctx, keys...)
}

// Expire устанавливает время жизни ключа
func (c *Client) Expire(ctx context.Context, key string, expiration time.Duration) error {
	ctx, cancel := c.timeoutContext(ctx)
	defer cancel()

	return c.conn.Expire(ctx, key, expiration)
}

// Ping проверяет соединение
func (c *Client) Ping(ctx context.Context) error {
	ctx, cancel := c.timeoutContext(ctx)
	defer cancel()

	return c.conn.Ping(ctx)
}

// Close закрывает соединение
func (c *Client) Close() error {
	return c.conn.Close()
}

func (c *Client) timeoutContext(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, c.timeout)
}

// EntityActorState состояние сущности для кэширования
type EntityActorState struct {
	EntityID     string                 `json:"entity_id"`
	EntityType   string                 `json:"entity_type"`
	State        map[string]float32     `json:"state"`
	ModelVersion string                 `json:"model_version"`
	LastUpdated  time.Time              `json:"last_updated"`
	Metadata     map[string]interface{} `json:"metadata"`
}

// GetActorState получает состояние актора из Redis
func (c *Client) GetActorState(ctx context.Context, entityID string) (*EntityActorState, error) {
	key := fmt.Sprintf("actor:state:%s", entityID)
	data, err := c.Get(ctx, key)
	if err != nil {
		return nil, err
	}

	var state EntityActorState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("failed to unmarshal state: %w", err)
	}

	return &state, nil
}

// SetActorState сохраняет состояние актора в Redis
func (c *Client) SetActorState(ctx context.Context, state *EntityActorState, ttl time.Duration) error {
	key := fmt.Sprintf("actor:state:%s", state.EntityID)
	data, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("failed to marshal state: %w", err)
	}

	return c.Set(ctx, key, data, ttl)
}

// GetModel загружает модель из Redis
func (c *Client) GetModel(ctx context.Context, modelID string) ([]byte, error) {
	key := fmt.Sprintf("actor:model:%s", modelID)
	return c.Get(ctx, key)
}

// SetModel сохраняет модель в Redis
func (c *Client) SetModel(ctx context.Context, modelID string, data []byte, ttl time.Duration) error {
	key := fmt.Sprintf("actor:model:%s", modelID)
	return c.Set(ctx, key, data, ttl)
}

// DeleteActorState удаляет состояние актора
func (c *Client) DeleteActorState(ctx context.Context, entityID string) error {
	key := fmt.Sprintf("actor:state:%s", entityID)
	return c.Delete(ctx, key)
}

// BatchGetStates получает состояния нескольких акторов
func (c *Client) BatchGetStates(ctx context.Context, entityIDs []string) (map[string]*EntityActorState, error) {
	result := make(map[string]*EntityActorState, len(entityIDs))

	for _, entityID := range entityIDs {
		state, err := c.GetActorState(ctx, entityID)
		if err != nil {
			// Игнорируем ошибки, возвращаем что получили
			continue
		}
		result[entityID] = state
	}

	return result, nil
}

// BatchSetStates сохраняет состояния нескольких акторов
func (c *Client) BatchSetStates(ctx context.Context, states []*EntityActorState, ttl time.Duration) error {
	for _, state := range states {
		if err := c.SetActorState(ctx, state, ttl); err != nil {
			return err
		}
	}
	return nil
}

// stubConn заглушка для тестирования (в production заменить на реальный Redis клиент)
type stubConn struct {
	data map[string][]byte
}

func (s *stubConn) Get(ctx context.Context, key string) ([]byte, error) {
	if val, ok := s.data[key]; ok {
		return val, nil
	}
	return nil, fmt.Errorf("key not found: %s", key)
}

func (s *stubConn) Set(ctx context.Context, key string, value []byte, expiration time.Duration) error {
	s.data[key] = value
	return nil
}

func (s *stubConn) Delete(ctx context.Context, key string) error {
	delete(s.data, key)
	return nil
}

func (s *stubConn) Exists(ctx context.Context, keys ...string) (int64, error) {
	count := int64(0)
	for _, key := range keys {
		if _, ok := s.data[key]; ok {
			count++
		}
	}
	return count, nil
}

func (s *stubConn) Expire(ctx context.Context, key string, expiration time.Duration) error {
	// В production здесь будет установка TTL
	return nil
}

func (s *stubConn) Ping(ctx context.Context) error {
	return nil
}

func (s *stubConn) Close() error {
	return nil
}

// Stats статистика Redis клиента
type Stats struct {
	Hits      int64 `json:"hits"`
	Misses    int64 `json:"misses"`
	Sets      int64 `json:"sets"`
	Deletes   int64 `json:"deletes"`
	Errors    int64 `json:"errors"`
	LatencyMs int64 `json:"latency_ms"`
}
