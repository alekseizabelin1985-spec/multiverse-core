package agent

import (
	"fmt"
	"strings"
	"regexp"
	"gopkg.in/yaml.v3"
)

// MarkdownParser для парсинга YAML-блоков из MD файлов
type MarkdownParser struct {
	yamlParser BlueprintParser
}

// NewMarkdownParser создает Markdown parser
func NewMarkdownParser() *MarkdownParser {
	return &MarkdownParser{
		yamlParser: NewYAMLParser(),
	}
}

// ParseFile парсит MD файл
func (p *MarkdownParser) ParseFile(path string) (*AgentBlueprint, error) {
	data, err := p.readFile(path)
	if err != nil {
		return nil, err
	}
	
	return p.ParseMD(data)
}

// ParseMD парсит MD содержание
func (p *MarkdownParser) ParseMD(content []byte) (*AgentBlueprint, error) {
	// Найдем YAML блок
	yamlContent := p.extractYAMLBlock(string(content))
	if yamlContent == "" {
		return nil, fmt.Errorf("no YAML block found in markdown")
	}
	
	return p.yamlParser.ParseYAML([]byte(yamlContent))
}

// extractYAMLBlock извлекает YAML блок из MD (между ```yaml ... ```)
func (p *MarkdownParser) extractYAMLBlock(content string) string {
	re := regexp.MustCompile("`{3,}`\\s*yaml\\s*\\n(.+?)\\n`{3,}`")
	matches := re.FindStringSubmatch(content)
	
	if len(matches) < 2 {
		return ""
	}
	
	return matches[1]
}

// readFile читает файл
func (p *MarkdownParser) readFile(path string) ([]byte, error) {
	data, err := readFile(path)
	if err != nil {
		return nil, err
	}
	return data, nil
}

// ParseMDWithFrontmatter парсит MD с frontmatter (YAML между ---)
func (p *MarkdownParser) ParseMDWithFrontmatter(content []byte) (*AgentBlueprint, error) {
	str := string(content)
	
	// Найдем frontmatter (между первыми ---)
	start := strings.Index(str, "---\n")
	if start == -1 {
		return nil, fmt.Errorf("no frontmatter found")
	}
	
	end := strings.Index(str[start+4:], "---\n")
	if end == -1 {
		return nil, fmt.Errorf("no closing frontmatter")
	}
	
	end += start + 4
	yamlContent := str[start+4 : end]
	
	return p.yamlParser.ParseYAML([]byte(yamlContent))
}

// ParseMDWithPrompt парсит MD с отдельным блоком промпта
func (p *MarkdownParser) ParseMDWithPrompt(content []byte) (*AgentBlueprint, error) {
	str := string(content)
	
	// Парсим frontmatter
	bp, err := p.ParseMDWithFrontmatter(content)
	if err != nil {
		return nil, err
	}
	
	// Найдем блоки промптов
	phase1Prompt := p.extractPromptBlock(str, "phase1_prompt")
	phase2Prompt := p.extractPromptBlock(str, "phase2_prompt")
	
	if phase1Prompt != "" {
		bp.Phase1Prompt = phase1Prompt
	}
	
	if phase2Prompt != "" {
		bp.Phase2Prompt = phase2Prompt
	}
	
	return bp, nil
}

// extractPromptBlock извлекает блок промпта
func (p *MarkdownParser) extractPromptBlock(content, name string) string {
	re := regexp.MustCompile(fmt.Sprintf("`{3,}`\\s*%s\\s*\\n(.+?)\\n`{3,}`", name))
	matches := re.FindStringSubmatch(content)
	
	if len(matches) < 2 {
		return ""
	}
	
	return matches[1]
}

// SerializeMD serializes blueprint to MD format
func (p *MarkdownParser) SerializeMD(bp *AgentBlueprint) ([]byte, error) {
	// Serializing to MD with YAML frontmatter
	markdown := fmt.Sprintf(`---
%s
---

# %s

%v
`,
		p.serializeToYAML(bp),
		bp.Name,
		bp.Description,
	)
	
	return []byte(markdown), nil
}

// serializeToYAML converts blueprint to YAML string
func (p *MarkdownParser) serializeToYAML(bp *AgentBlueprint) (string) {
	data, err := yaml.Marshal(bp)
	if err != nil {
		return ""
	}
	return string(data)
}

// IsMDFile проверяет, является ли файл MD
func (p *MarkdownParser) IsMDFile(filename string) bool {
	return strings.HasSuffix(filename, ".md") || strings.HasSuffix(filename, ".MD")
}
