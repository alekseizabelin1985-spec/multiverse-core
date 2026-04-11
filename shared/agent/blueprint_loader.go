package agent

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// yamlParser - реализация BlueprintParser для YAML
type yamlParser struct{}

// NewYAMLParser создает новый YAML парсер
func NewYAMLParser() BlueprintParser {
	return &yamlParser{}
}

// ParseFile парсит файл блупринта
func (p *yamlParser) ParseFile(path string) (*AgentBlueprint, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}
	return p.ParseYAML(data)
}

// ParseYAML парсит YAML содержание
func (p *yamlParser) ParseYAML(data []byte) (*AgentBlueprint, error) {
	var bp AgentBlueprint
	if err := yaml.Unmarshal(data, &bp); err != nil {
		return nil, fmt.Errorf("parse yaml: %w", err)
	}
	return &bp, nil
}

// Validate валидирует блупринт
func (p *yamlParser) Validate(bp *AgentBlueprint) error {
	if bp.Name == "" {
		return fmt.Errorf("blueprint name is required")
	}
	
	if bp.Version == "" {
		return fmt.Errorf("blueprint version is required")
	}
	
	if bp.Trigger.Type == "" {
		return fmt.Errorf("blueprint trigger type is required")
	}
	
	// Валидация LLM config
	if bp.LLM.Model == "" {
		return fmt.Errorf("blueprint LLM model is required")
	}
	
	return nil
}

// Serialize serializes blueprint to YAML
func (p *yamlParser) Serialize(bp *AgentBlueprint) ([]byte, error) {
	return yaml.Marshal(bp)
}

// LoadBlueprintFromDir загружает все блупринты из директории
func LoadBlueprintsFromDir(dir string) ([]*AgentBlueprint, error) {
	var blueprints []*AgentBlueprint
	
	parser := NewYAMLParser()
	
	// Читаем все .md и .yaml файлы
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read directory: %w", err)
	}
	
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		
		name := entry.Name()
		
		// Проверяем расширение
		if !strings.HasSuffix(name, ".md") && !strings.HasSuffix(name, ".yaml") && !strings.HasSuffix(name, ".yml") {
			continue
		}
		
		path := fmt.Sprintf("%s/%s", dir, name)
		
		bp, err := parser.ParseFile(path)
		if err != nil {
			// Пропускаем невалидные файлы, но логируем
			fmt.Printf("Warning: skipping invalid blueprint %s: %v\n", name, err)
			continue
		}
		
		blueprints = append(blueprints, bp)
	}
	
	return blueprints, nil
}
