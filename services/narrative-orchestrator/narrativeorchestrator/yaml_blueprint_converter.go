// services/narrativeorchestrator/yaml_blueprint_converter.go

package narrativeorchestrator

import (
	"encoding/json"
	"fmt"
	"io"
	"multiverse-core.io/shared/agent"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// YAMLBlueprintConverter конвертирует gm_*.yaml файлы в AgentBlueprint
type YAMLBlueprintConverter struct {
	configDir string
}

// YAMLConfig структура для парсинга gm_*.yaml файлов
type YAMLConfig struct {
	ScopeType     string   `yaml:"scope_type"`
	FocusEntities []string `yaml:"focus_entities"`
	TimeWindow    string   `yaml:"time_window"`
	ContextDepth  struct {
		Canon    int `yaml:"canon"`
		History  int `yaml:"history"`
		Entities int `yaml:"entities"`
	} `yaml:"context_depth"`
	Include struct {
		WorldFacts       bool `yaml:"world_facts"`
		EntityEmotions   bool `yaml:"entity_emotions"`
		LocationDetails  bool `yaml:"location_details"`
		TemporalContext  bool `yaml:"temporal_context"`
	} `yaml:"include"`
	Triggers struct {
		TimeIntervalMS    int      `yaml:"time_interval_ms"`
		MaxEvents         int      `yaml:"max_events"`
		NarrativeTriggers []string `yaml:"narrative_triggers"`
	} `yaml:"triggers"`
}

// NewYAMLBlueprintConverter создает новый конвертер
func NewYAMLBlueprintConverter(configDir string) *YAMLBlueprintConverter {
	return &YAMLBlueprintConverter{
		configDir: configDir,
	}
}

// ConvertFile конвертирует один gm_*.yaml файл в AgentBlueprint
func (c *YAMLBlueprintConverter) ConvertFile(filePath string) (*agent.AgentBlueprint, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("read file %s: %w", filePath, err)
	}

	return c.ConvertBytes(data)
}

// ConvertBytes конвертирует YAML данные в AgentBlueprint
func (c *YAMLBlueprintConverter) ConvertBytes(data []byte) (*agent.AgentBlueprint, error) {
	// Парсим YAML через yaml.v3
	var cfg YAMLConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("unmarshal YAML: %w", err)
	}

	// Извлекаем scope_type
	scopeType := cfg.ScopeType
	if scopeType == "" {
		scopeType = "default"
	}

	// Извлекаем time_window
	timeWindow := cfg.TimeWindow
	if timeWindow == "" {
		timeWindow = "10m"
	}

	// Извлекаем triggers
	triggers := cfg.Triggers.NarrativeTriggers
	if len(triggers) == 0 {
		triggers = []string{
			"combat.start",
			"player.entered_boss_room",
			"ritual.completed",
		}
	}

	// Извлекаем context_depth
	canonDepth := cfg.ContextDepth.Canon
	if canonDepth == 0 {
		canonDepth = 3
	}
	historyDepth := cfg.ContextDepth.History
	if historyDepth == 0 {
		historyDepth = 5
	}

	// Извлекаем include flags
	includeWorldFacts := cfg.Include.WorldFacts
	includeEntityEmotions := cfg.Include.EntityEmotions
	includeLocationDetails := cfg.Include.LocationDetails
	includeTemporalContext := cfg.Include.TemporalContext

	// Формируем Phase1Prompt (механика)
	phase1Prompt := fmt.Sprintf(`Ты — Game Master области %s.

### КОНТЕКСТ МИРА
Глубина канона: %d
Глубина истории: %d
Временное окно: %s

### ОБЛАСТЬ
Тип: %s

### ВКЛЮЧЕНИЕ КОНТЕКСТА
- Факты мира: %v
- Эмоции сущностей: %v
- Детали локаций: %v
- Временной контекст: %v

### ТРИГГЕРЫ
%s

### ПРАВИЛА
1. Обрабатывай только события, соответствующие триггерам
2. Используй механику (Phase 1) для принятия решений
3. Генерируй нарратив (Phase 2) на основе решений
4. Публикуй решения как события`,
		scopeType,
		canonDepth,
		historyDepth,
		timeWindow,
		scopeType,
		includeWorldFacts,
		includeEntityEmotions,
		includeLocationDetails,
		includeTemporalContext,
		strings.Join(triggers, ", "),
	)

	// Формируем Phase2Prompt (нарратив)
	phase2Prompt := `Опиши событие в атмосферной манере.

### ОБЛАСТЬ
Тип: ` + scopeType + `

### КОНТЕКСТ
Используй факты мира, эмоции сущностей, детали локаций.

### СОБЫТИЕ
{trigger_event}

### МЕХАНИКА
{phase1_result}

Будь креативным, описывай звуки, запахи, ощущения. 1-3 предложения.`

	// Определяем TTL на основе scope_type
	ttl := c.determineTTL(scopeType)

	return &agent.AgentBlueprint{
		Name:        "narrative-gm-" + scopeType,
		Version:     "1.0",
		Description: "Narrative GM agent for " + scopeType + " scope",
		Trigger: agent.BlueprintTrigger{
			Type:      "event",
			EventName: "batch.process",
		},
		Constraints: agent.BlueprintConstraints{
			MaxInstances: 1, // Один GM на scope
		},
		Phase1Prompt: phase1Prompt,
		Phase2Prompt: phase2Prompt,
		TTL:          ttl,
		Type:         scopeType,
	}, nil
}

// ConvertDir конвертирует все gm_*.yaml файлы из директории
func (c *YAMLBlueprintConverter) ConvertDir() ([]*agent.AgentBlueprint, error) {
	entries, err := os.ReadDir(c.configDir)
	if err != nil {
		return nil, fmt.Errorf("read config dir %s: %w", c.configDir, err)
	}

	var blueprints []*agent.AgentBlueprint

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		fileName := entry.Name()
		if !strings.HasPrefix(fileName, "gm_") || !strings.HasSuffix(fileName, ".yaml") {
			continue
		}

		filePath := filepath.Join(c.configDir, fileName)
		bp, err := c.ConvertFile(filePath)
		if err != nil {
			return nil, fmt.Errorf("convert %s: %w", fileName, err)
		}

		blueprints = append(blueprints, bp)
	}

	return blueprints, nil
}

// helper: извлекает строковое поле из YAML
func (c *YAMLBlueprintConverter) extractField(data []byte, fieldName string) string {
	// Устаревший метод — оставлен для обратной совместимости
	var cfg YAMLConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return ""
	}

	switch fieldName {
	case "scope_type":
		return cfg.ScopeType
	case "time_window":
		return cfg.TimeWindow
	default:
		return ""
	}
}

// helper: извлекает список строк
func (c *YAMLBlueprintConverter) extractStringList(data []byte, fieldName string) []string {
	// Устаревший метод — оставлен для обратной совместимости
	var cfg YAMLConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil
	}

	switch fieldName {
	case "narrative_triggers":
		return cfg.Triggers.NarrativeTriggers
	default:
		return nil
	}
}

// helper: извлекает int поле
func (c *YAMLBlueprintConverter) extractIntField(data []byte, fieldName string) int {
	// Устаревший метод — оставлен для обратной совместимости
	var cfg YAMLConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return 0
	}

	switch fieldName {
	case "canon":
		return cfg.ContextDepth.Canon
	case "history":
		return cfg.ContextDepth.History
	default:
		return 0
	}
}

// helper: извлекает bool поле
func (c *YAMLBlueprintConverter) extractBoolField(data []byte, fieldName string) bool {
	// Устаревший метод — оставлен для обратной совместимости
	var cfg YAMLConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return false
	}

	switch fieldName {
	case "world_facts":
		return cfg.Include.WorldFacts
	case "entity_emotions":
		return cfg.Include.EntityEmotions
	case "location_details":
		return cfg.Include.LocationDetails
	case "temporal_context":
		return cfg.Include.TemporalContext
	default:
		return false
	}
}

// helper: определяет TTL на основе scope_type
func (c *YAMLBlueprintConverter) determineTTL(scopeType string) string {
	switch scopeType {
	case "world":
		return "24h"
	case "region":
		return "12h"
	case "city":
		return "6h"
	case "group":
		return "2h"
	case "location":
		return "1h"
	default:
		return "10m"
	}
}

// helper: определяет уровень агента на основе scope_type
func (c *YAMLBlueprintConverter) determineLevel(scopeType string) agent.AgentLevel {
	switch scopeType {
	case "world":
		return agent.LevelGlobal
	case "region":
		return agent.LevelDomain
	case "city":
		return agent.LevelDomain
	case "group":
		return agent.LevelTask
	case "location":
		return agent.LevelObject
	default:
		return agent.LevelTask
	}
}

// LoadBlueprints загружает все блупринты из YAML файлов и регистрирует их в router
func (c *YAMLBlueprintConverter) LoadBlueprints(router *agent.Router) error {
	blueprints, err := c.ConvertDir()
	if err != nil {
		return fmt.Errorf("convert blueprints: %w", err)
	}

	for _, bp := range blueprints {
		if err := router.RegisterBlueprint(bp); err != nil {
			return fmt.Errorf("register blueprint %s: %w", bp.Name, err)
		}
	}

	return nil
}

// SaveBlueprints сохраняет AgentBlueprint в MD файлы (для документации)
func (c *YAMLBlueprintConverter) SaveBlueprints(blueprints []*agent.AgentBlueprint, outputDir string) error {
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("create output dir: %w", err)
	}

	for _, bp := range blueprints {
		fileName := fmt.Sprintf("blueprint_%s.md", bp.Name)
		filePath := filepath.Join(outputDir, fileName)

		content := fmt.Sprintf(`# Agent Blueprint: %s

**Name**: %s
**Version**: %s
**Description**: %s
**TTL**: %s
**Max Instances**: %d
**Type**: %s

---

## Trigger

- **Type**: %s
- **Event Name**: %s

### Conditions
`,
			bp.Name, bp.Name, bp.Version, bp.Description,
			bp.TTL, bp.Constraints.MaxInstances, bp.Type,
			bp.Trigger.Type, bp.Trigger.EventName,
		)

		for _, cond := range bp.Trigger.Conditions {
			content += fmt.Sprintf("- **%s** %s %d\n", cond.Field, cond.Operator, cond.Value)
		}

		content += "\n---\n\n## Phase 1 Prompt (Mechanics)\n\n" + bp.Phase1Prompt + "\n"
		content += "\n---\n\n## Phase 2 Prompt (Narrative)\n\n" + bp.Phase2Prompt + "\n"

		if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
			return fmt.Errorf("save blueprint %s: %w", bp.Name, err)
		}
	}

	return nil
}

// MarkdownBlueprintParser парсит MD/JSON блупринты в AgentBlueprint
// (не конфликтует с agent.BlueprintParser интерфейсом)
type MarkdownBlueprintParser struct{}

// NewMarkdownBlueprintParser создает новый парсер
func NewMarkdownBlueprintParser() *MarkdownBlueprintParser {
	return &MarkdownBlueprintParser{}
}

// ParseFile парсит MD или JSON файл блупринта
func (p *MarkdownBlueprintParser) ParseFile(filePath string) (*agent.AgentBlueprint, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}

	ext := strings.ToLower(filepath.Ext(filePath))
	if ext == ".json" {
		return p.ParseJSON(data)
	}

	return p.ParseMD(data)
}

// ParseMD парсит MD файл блупринта
func (p *MarkdownBlueprintParser) ParseMD(data []byte) (*agent.AgentBlueprint, error) {
	content := string(data)

	bp := &agent.AgentBlueprint{}

	// Извлекаем Name
	bp.Name = extractMDField(content, "# Agent Blueprint:")

	// Извлекаем Version
	bp.Version = extractMDField(content, "**Version**:")

	// Извлекаем Description
	bp.Description = extractMDField(content, "**Description**:")

	// Извлекаем TTL
	bp.TTL = extractMDField(content, "**TTL**:")

	// Извлекаем Type
	bp.Type = extractMDField(content, "**Type**:")

	// Извлекаем MaxInstances
	bp.Constraints.MaxInstances = extractMDIntField(content, "**Max Instances**:")

	// Извлекаем Phase1Prompt
	bp.Phase1Prompt = extractMDSection(content, "## Phase 1 Prompt (Mechanics)")

	// Извлекаем Phase2Prompt
	bp.Phase2Prompt = extractMDSection(content, "## Phase 2 Prompt (Narrative)")

	// Извлекаем Trigger
	bp.Trigger.Type = extractMDField(content, "**Type**:")
	bp.Trigger.EventName = extractMDField(content, "**Event Name**:")

	return bp, nil
}

// ParseJSON парсит JSON файл блупринта
func (p *MarkdownBlueprintParser) ParseJSON(data []byte) (*agent.AgentBlueprint, error) {
	// Используем стандартный json.Unmarshal
	var bp agent.AgentBlueprint
	if err := parseJSONBlueprint(data, &bp); err != nil {
		return nil, fmt.Errorf("parse JSON blueprint: %w", err)
	}

	return &bp, nil
}

// parseJSONBlueprint парсит JSON в AgentBlueprint
func parseJSONBlueprint(data []byte, bp *agent.AgentBlueprint) error {
	return json.Unmarshal(data, bp)
}

// ParseString парсит строку блупринта (предполагается MD формат)
func (p *MarkdownBlueprintParser) ParseString(s string) (*agent.AgentBlueprint, error) {
	return p.ParseMD([]byte(s))
}

// extractMDField извлекает значение поля из MD
func extractMDField(content, prefix string) string {
	idx := strings.Index(content, prefix)
	if idx == -1 {
		return ""
	}

	idx += len(prefix)
	// Пропускаем пробелы
	for idx < len(content) && content[idx] == ' ' {
		idx++
	}

	end := idx
	for end < len(content) && content[end] != '\n' {
		end++
	}

	return strings.TrimSpace(content[idx:end])
}

// extractMDIntField извлекает int поле из MD
func extractMDIntField(content, prefix string) int {
	valStr := extractMDField(content, prefix)
	val := 0
	fmt.Sscanf(valStr, "%d", &val)
	return val
}

// extractMDSection извлекает секцию из MD
func extractMDSection(content, section string) string {
	idx := strings.Index(content, section)
	if idx == -1 {
		return ""
	}

	idx += len(section)
	start := idx

	// Ищем следующую секцию ##
	nextSection := strings.Index(content[idx:], "\n## ")
	if nextSection == -1 {
		return strings.TrimSpace(content[start:])
	}

	return strings.TrimSpace(content[start : idx+nextSection])
}

// ParseBlueprintFile — алиас для ParseFile (для обратной совместимости)
func ParseBlueprintFile(filePath string) (*agent.AgentBlueprint, error) {
	p := NewMarkdownBlueprintParser()
	return p.ParseFile(filePath)
}

// ParseBlueprintBytes — парсит байты блупринта
func ParseBlueprintBytes(data []byte, format string) (*agent.AgentBlueprint, error) {
	p := NewMarkdownBlueprintParser()

	switch format {
	case "json":
		return p.ParseJSON(data)
	case "md", "markdown":
		return p.ParseMD(data)
	default:
		return p.ParseMD(data)
	}
}

// ParseBlueprint — парсит блупринт из io.Reader
func ParseBlueprint(r io.Reader) (*agent.AgentBlueprint, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("read blueprint: %w", err)
	}

	p := NewMarkdownBlueprintParser()
	return p.ParseMD(data)
}
