// internal/rules/engine.go
package rules

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"strconv"
	"strings"
	"sync"
	"time"

	"multiverse-core/internal/minio"
)

// Engine движок применения правил
type Engine struct {
	mu          sync.RWMutex
	minioClient *minio.MinIOOfficialClient
	bucketName  string
	cache       *LRUCache // LRU кэш для горячих правил
	rand        *rand.Rand
	logger      Logger
}

// Logger интерфейс для логгера
type Logger interface {
	Printf(format string, v ...interface{})
}

type defaultLogger struct{}

func (l *defaultLogger) Printf(format string, v ...interface{}) {
	fmt.Printf(format, v...)
}

// LRUCache простой LRU кэш
type LRUCache struct {
	mu       sync.Mutex
	capacity int
	items    map[string]*cacheItem
	head     *cacheItem
	tail     *cacheItem
}

type cacheItem struct {
	key      string
	value    *Rule
	prev     *cacheItem
	next     *cacheItem
	lastUsed time.Time
}

// NewLRUCache создает LRU кэш
func NewLRUCache(capacity int) *LRUCache {
	return &LRUCache{
		capacity: capacity,
		items:    make(map[string]*cacheItem),
	}
}

// Get получает элемент из кэша
func (c *LRUCache) Get(key string) (*Rule, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	item, exists := c.items[key]
	if !exists {
		return nil, false
	}

	// Перемещаем в начало (most recently used)
	c.moveToFront(item)
	item.lastUsed = time.Now()

	return item.value, true
}

// Put добавляет элемент в кэш
func (c *LRUCache) Put(key string, value *Rule) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Если уже есть - обновляем
	if item, exists := c.items[key]; exists {
		item.value = value
		c.moveToFront(item)
		item.lastUsed = time.Now()
		return
	}

	// Создаем новый элемент
	item := &cacheItem{
		key:      key,
		value:    value,
		lastUsed: time.Now(),
	}

	// Добавляем в начало
	c.addToFront(item)
	c.items[key] = item

	// Если превысили capacity - удаляем последний
	if len(c.items) > c.capacity {
		c.removeTail()
	}
}

// Remove удаляет элемент из кэша
func (c *LRUCache) Remove(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	item, exists := c.items[key]
	if !exists {
		return
	}

	c.removeItem(item)
	delete(c.items, key)
}

func (c *LRUCache) addToFront(item *cacheItem) {
	if c.head == nil {
		c.head = item
		c.tail = item
		item.prev = nil
		item.next = nil
	} else {
		item.next = c.head
		c.head.prev = item
		item.prev = nil
		c.head = item
	}
}

func (c *LRUCache) moveToFront(item *cacheItem) {
	if item == c.head {
		return
	}

	// Удаляем из текущего места
	c.removeItem(item)

	// Добавляем в начало
	c.addToFront(item)
}

func (c *LRUCache) removeItem(item *cacheItem) {
	if item.prev != nil {
		item.prev.next = item.next
	} else {
		c.head = item.next
	}

	if item.next != nil {
		item.next.prev = item.prev
	} else {
		c.tail = item.prev
	}

	item.prev = nil
	item.next = nil
}

func (c *LRUCache) removeTail() {
	if c.tail == nil {
		return
	}

	item := c.tail
	c.removeItem(item)
	delete(c.items, item.key)
}

// NewEngine создает новый движок правил
func NewEngine(minioClient *minio.MinIOOfficialClient, bucketName string, cacheSize int) *Engine {
	return &Engine{
		minioClient: minioClient,
		bucketName:  bucketName,
		cache:       NewLRUCache(cacheSize),
		rand:        rand.New(rand.NewSource(time.Now().UnixNano())),
		logger:      &defaultLogger{},
	}
}

// Apply применяет правило к состоянию и возвращает результат
func (e *Engine) Apply(ruleID string, state map[string]float32, modifiers []map[string]interface{}) (*RuleResult, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	// Валидация входных данных
	if ruleID == "" {
		return nil, fmt.Errorf("rule ID cannot be empty")
	}
	if state == nil {
		return nil, fmt.Errorf("state cannot be nil")
	}

	// Получаем правило из кэша или хранилища
	rule, err := e.getRule(ruleID)
	if err != nil {
		return nil, fmt.Errorf("get rule: %w", err)
	}

	// Проверяем требования
	if err := e.checkRequirements(rule, state); err != nil {
		return &RuleResult{
			RuleID:         ruleID,
			RuleVersion:    rule.Version,
			Success:        false,
			MechanicalOnly: true,
			AppliedAt:      time.Now(),
		}, fmt.Errorf("requirements not met: %w", err)
	}

	// Вычисляем базовый бросок
	diceResult, err := e.rollDice(rule.MechanicalCore.DiceFormula, state)
	if err != nil {
		return nil, fmt.Errorf("roll dice: %w", err)
	}

	// Применяем модификаторы
	total := diceResult
	var appliedModifiers []AppliedModifier

	for _, mod := range rule.MechanicalCore.ContextualModifiers {
		applied := e.evaluateCondition(mod.Condition, state, modifiers)
		modifier := AppliedModifier{
			ID:         mod.ID,
			Name:       mod.Name,
			Value:      mod.Modifier,
			Condition:  mod.Condition,
			WasApplied: applied,
		}

		if applied {
			total += mod.Modifier
		}
		appliedModifiers = append(appliedModifiers, modifier)
	}

	// Проверяем успех
	success, critical := e.checkSuccessThreshold(total, rule.MechanicalCore.SuccessThreshold, diceResult)

	// Создаем результат
	result := &RuleResult{
		RuleID:         ruleID,
		RuleVersion:    rule.Version,
		DiceRoll:       diceResult,
		DiceFormula:    rule.MechanicalCore.DiceFormula,
		Total:          total,
		Success:        success,
		Critical:       critical,
		Context:        state,
		Modifiers:      appliedModifiers,
		SensoryEffects: rule.MechanicalCore.SensoryEffects,
		StateChanges:   rule.MechanicalCore.StateChanges,
		MechanicalOnly: true, // Только механика, нарратив добавляет GM
		AppliedAt:      time.Now(),
	}

	return result, nil
}

// rollDice бросает кубики по формуле
func (e *Engine) rollDice(formula string, state map[string]float32) (int, error) {
	if formula == "" {
		return 0, fmt.Errorf("dice formula cannot be empty")
	}

	formula = strings.TrimSpace(formula)

	// Парсим формулу: "2d6+3", "d20", "d10+charisma"
	var numDice int = 1
	var dieSides int = 20
	var bonus int = 0

	// Разделяем на части
	parts := strings.Split(formula, "d")
	if len(parts) == 2 {
		// Есть количество кубиков
		if parts[0] != "" {
			n, err := strconv.Atoi(parts[0])
			if err != nil {
				return 0, fmt.Errorf("invalid dice count: %s", parts[0])
			}
			numDice = n
		}

		// Парсим количество граней и бонус
		rightParts := strings.Split(parts[1], "+")
		dieSides, _ = strconv.Atoi(rightParts[0])
		if dieSides <= 0 {
			return 0, fmt.Errorf("invalid die sides: %d", dieSides)
		}

		if len(rightParts) > 1 {
			// Проверяем, не имя ли это характеристики
			bonusStr := rightParts[1]
			if bonusVal, err := strconv.Atoi(bonusStr); err == nil {
				bonus = bonusVal
			} else {
				// Это имя характеристики из state
				if val, ok := state[bonusStr]; ok {
					bonus = int(val)
				}
			}
		}
	} else if strings.HasPrefix(formula, "d") {
		// Просто кубик: "d20"
		dieSides, _ = strconv.Atoi(formula[1:])
		if dieSides <= 0 {
			return 0, fmt.Errorf("invalid die sides: %s", formula[1:])
		}
	}

	// Бросаем кубики
	total := 0
	for i := 0; i < numDice; i++ {
		roll := e.rand.Intn(dieSides) + 1
		total += roll
	}

	// Добавляем бонус
	total += bonus

	// Проверяем критические значения
	critical := (total == dieSides*numDice+bonus) || (total == 1+bonus)
	_ = critical

	return total, nil
}

// evaluateCondition вычисляет условие
func (e *Engine) evaluateCondition(condition string, state map[string]float32, modifiers []map[string]interface{}) bool {
	if condition == "" {
		return false
	}

	condition = strings.TrimSpace(condition)

	// Простой парсер условий: "health > 50", "level >= 10"
	parts := strings.Fields(condition)
	if len(parts) < 3 {
		return false
	}

	varName := parts[0]
	op := parts[1]
	valueStr := strings.Join(parts[2:], " ")

	stateVal, exists := state[varName]
	if !exists {
		return false
	}

	val, err := strconv.ParseFloat(valueStr, 32)
	if err != nil {
		return false
	}

	switch op {
	case ">":
		return stateVal > float32(val)
	case "<":
		return stateVal < float32(val)
	case ">=":
		return stateVal >= float32(val)
	case "<=":
		return stateVal <= float32(val)
	case "=", "==":
		return stateVal == float32(val)
	case "!=":
		return stateVal != float32(val)
	default:
		return false
	}
}

// checkSuccessThreshold проверяет порог успеха
func (e *Engine) checkSuccessThreshold(total int, threshold string, diceRoll int) (bool, bool) {
	if threshold == "" {
		return true, false
	}

	threshold = strings.TrimSpace(threshold)
	parts := strings.Fields(threshold)

	if len(parts) >= 3 && parts[0] == "total" {
		op := parts[1]
		val, err := strconv.Atoi(parts[2])
		if err != nil {
			return true, false
		}

		var success bool
		switch op {
		case ">=":
			success = total >= val
		case ">":
			success = total > val
		case "<=":
			success = total <= val
		case "<":
			success = total < val
		case "=", "==":
			success = total == val
		default:
			success = true
		}

		// Критический успех/провал
		critical := (diceRoll == 20) || (diceRoll == 1)
		return success, critical
	}

	return true, false
}

// checkRequirements проверяет требования правила
func (e *Engine) checkRequirements(rule *Rule, state map[string]float32) error {
	for _, req := range rule.MechanicalCore.Requirements {
		val, exists := state[req.Attribute]
		if !exists {
			return fmt.Errorf("missing required attribute: %s", req.Attribute)
		}

		if val < req.MinValue || val > req.MaxValue {
			return fmt.Errorf("attribute %s value %.2f out of range [%.2f, %.2f]",
				req.Attribute, val, req.MinValue, req.MaxValue)
		}
	}

	return nil
}

// getRule получает правило из кэша или хранилища
func (e *Engine) getRule(ruleID string) (*Rule, error) {
	// Проверяем кэш
	if rule, exists := e.cache.Get(ruleID); exists {
		return rule, nil
	}

	// Загружаем из MinIO
	rule, err := e.loadRuleFromStorage(ruleID)
	if err != nil {
		return nil, err
	}

	// Кэшируем
	e.cache.Put(ruleID, rule)

	return rule, nil
}

// loadRuleFromStorage загружает правило из MinIO
func (e *Engine) loadRuleFromStorage(ruleID string) (*Rule, error) {
	if e.minioClient == nil {
		return nil, fmt.Errorf("minio client not initialized")
	}

	data, err := e.minioClient.GetObject(e.bucketName, fmt.Sprintf("rules/%s.json", ruleID))
	if err != nil {
		return nil, fmt.Errorf("failed to get rule from storage: %w", err)
	}

	if len(data) == 0 {
		return nil, fmt.Errorf("rule not found: %s", ruleID)
	}

	var rule Rule
	if err := json.Unmarshal(data, &rule); err != nil {
		return nil, fmt.Errorf("failed to unmarshal rule: %w", err)
	}

	return &rule, nil
}

// SaveRule сохраняет правило в хранилище и кэш
func (e *Engine) SaveRule(rule *Rule) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if rule == nil || rule.ID == "" {
		return fmt.Errorf("rule cannot be nil and must have ID")
	}

	data, err := json.Marshal(rule)
	if err != nil {
		return fmt.Errorf("failed to marshal rule: %w", err)
	}

	objectKey := fmt.Sprintf("rules/%s.json", rule.ID)
	if err := e.minioClient.PutObject(e.bucketName, objectKey, strings.NewReader(string(data)), int64(len(data))); err != nil {
		return fmt.Errorf("failed to save rule: %w", err)
	}

	// Кэшируем
	e.cache.Put(rule.ID, rule)

	return nil
}

// DeleteRule удаляет правило
func (e *Engine) DeleteRule(ruleID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.cache.Remove(ruleID)
	// В production: удалить из MinIO

	return nil
}

// GetCacheStats возвращает статистику кэша
func (e *Engine) GetCacheStats() CacheStats {
	e.mu.RLock()
	defer e.mu.RUnlock()

	return CacheStats{
		CachedRules: len(e.cache.items),
		Capacity:    e.cache.capacity,
	}
}

// CacheStats статистика кэша
type CacheStats struct {
	CachedRules int `json:"cached_rules"`
	Capacity    int `json:"capacity"`
}
