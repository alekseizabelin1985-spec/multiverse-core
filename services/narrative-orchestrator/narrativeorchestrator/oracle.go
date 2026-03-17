// services/narrativeorchestrator/oracle.go

package narrativeorchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"multiverse-core.io/shared/oracle"
	"strings"
	"time"
)

type OracleResponse struct {
	Narrative string                   `json:"narrative"`
	Mood      []string                 `json:"mood,omitempty"`
	NewEvents []map[string]interface{} `json:"new_events"`
}

// PromptInput — данные для генерации промта.
type PromptInput struct {
	WorldContext    string
	ScopeID         string
	ScopeType       string
	EntitiesContext string
	EventClusters   []EventCluster
	TimeContext     string
	TriggerEvent    string
}

// BuildPrompt генерирует промт для Oracle.
func BuildPrompt(input PromptInput) (string, string) {
	system := strings.TrimSpace(`
Ты — Повествователь Мира. Твоя задача — развивать историю естественно, иммерсивно и поэтично.

### КОНТЕКСТ МИРА
` + input.WorldContext + `

### ОБЛАСТЬ ПОВЕСТВОВАНИЯ
ID области: ` + input.ScopeID + `
Тип области: ` + input.ScopeType + `
Сущности в области:
` + input.EntitiesContext + `

### ТРЕБОВАНИЯ К ФОРМАТУ ОТВЕТА

Отвечай СТРОГО валидным JSON без дополнительного текста, пояснений или блоков кода.

Структура ответа:
{
  "narrative": "1–3 предложения повествования",
  "mood": ["Напрежение", "настроение2","Угнетение"],
  "new_events": [
    {
      "event_type": "тип.события",
      "timestamp": "2026-02-13T14:06:00Z",
      "source": "player:kain-777",
      "world_id": "pain-realm",
      "scope_id": "player:kain-777",
      "payload": {
        "description": "Краткое описание действия",
        "любые_поля": "в зависимости от контекста"
      }
    }
  ]
}

ВАЖНЫЕ ПРАВИЛА:
• События должны быть связаны с областью 
• Поле "payload" — объект с произвольной структурой. Могут быть ЛЮБЫЕ поля, релевантные событию (skill_id, effect, target, duration и т.д.). Всегда возвращай валидный объект {}.
• Поле "scope_id" ОБЯЗАТЕЛЬНО присутствует как свойство. Значение может быть пустой строкой "".
• "mood" — массив строк (может быть пустым: []).
• "new_events" — массив до 3 событий.
• timestamp — ISO 8601 UTC (оканчивается на Z).
• Ответ должен начинаться с { и заканчиваться }.
• Твой ответ будет передан НАПРЯМУЮ в JSON-парсер. Любая синтаксическая ошибка (комментарии //, многоточия ..., кавычки-ёлочки «») сломает систему.

### ЖЁСТКИЕ ОГРАНИЧЕНИЯ
— МАКСИМУМ 3 события в new_events. НИ ОДНОГО БОЛЬШЕ.
— КАЖДОЕ событие должно иметь:
  • event_type в формате snake_case (например: "environment.sound", "player.reacted")
  • timestamp в UTC (оканчивается на Z)
  • source: "player:kain-777"
  • world_id: "pain-realm" (ОБЯЗАТЕЛЬНО, не пустая строка)
  • scope_id: "player:kain-777" (ОБЯЗАТЕЛЬНО присутствует как свойство. Значение может быть пустой строкой "")
  • payload: объект с релевантными полями (НЕ атомарные действия вроде "повернул ручку")
— События должны быть СЕМАНТИЧЕСКИМИ (звук за дверью, появление сущности), а не анимацией шагов.
— Если не можешь уложиться в 3 события — выбери САМОЕ ВАЖНОЕ.
— Ответ ДОЛЖЕН завершаться закрывающей скобкой } без обрезки.

`)

	user := strings.TrimSpace(`
### ВРЕМЕННОЙ КОНТЕКСТ
` + input.TimeContext + `

### НАКОПЛЕННЫЕ СОБЫТИЯ
` + buildEventClusters(input.EventClusters) + `

### СОБЫТИЕ-ТРИГГЕР
` + input.TriggerEvent + `

### ЗАДАЧА
Подумай: что *логично* происходит дальше?
— Учитывай факты, характеры, обстановку.
— Даже если событий мало — мир живёт.
— Используй стилевые модификаторы: «внезапно», «плавно», «тревожно».

### СОЗДАНИЕ СОБЫТИЙ
Генерируй события ТОЛЬКО в формате, описанном выше. Не добавляй полей вне спецификации.
`)
	log.Println("system:", system)
	log.Println("user:", user)
	return system, user
}

func CallOracle(ctx context.Context, systemPrompt, userPrompt string) (*OracleResponse, error) {
	client := oracle.NewClient()
	content, err := client.CallStructured(ctx, systemPrompt, userPrompt)
	if err != nil {
		return nil, fmt.Errorf("oracle call failed: %w", err)
	}
	if content == "" {
		return nil, fmt.Errorf("empty content")
	}

	log.Printf("Oracle response: %s", content)
	var result OracleResponse
	if err := json.Unmarshal([]byte(content), &result); err != nil {
		return nil, fmt.Errorf("invalid JSON: %s", content)
	}
	if result.Narrative == "" {
		return nil, fmt.Errorf("empty narrative")
	}
	return &result, nil
}

// cleanJSONResponse снимает markdown code block, если LLM обернул JSON в него.
func cleanJSONResponse(s string) string {
	s = strings.TrimSpace(s)
	// Снять ```json ... ``` или ``` ... ```
	if strings.HasPrefix(s, "```") {
		// Отрезать первую строку (```json или ```)
		idx := strings.Index(s, "\n")
		if idx != -1 {
			s = s[idx+1:]
		}
		// Отрезать финальный ```
		if end := strings.LastIndex(s, "```"); end != -1 {
			s = s[:end]
		}
		s = strings.TrimSpace(s)
	}
	return s
}

// CallOracleStructured — новая версия CallOracle на основе PromptSections.
// Использует BuildStructuredPrompt + CallStructuredJSON + валидацию на стороне Go.
// Старая CallOracle остаётся без изменений.
func CallOracleStructured(ctx context.Context, sections PromptSections) (*OracleResponse, error) {
	systemPrompt, userPrompt := BuildStructuredPrompt(sections)

	client := oracle.NewClient()
	content, err := client.CallStructuredJSON(ctx, systemPrompt, userPrompt)
	if err != nil {
		return nil, fmt.Errorf("oracle call failed: %w", err)
	}
	if content == "" {
		return nil, fmt.Errorf("empty content from oracle")
	}

	log.Printf("Oracle structured response: %s", content)

	// Fallback: очистить markdown code block если есть
	cleaned := cleanJSONResponse(content)

	var result OracleResponse
	if err := json.Unmarshal([]byte(cleaned), &result); err != nil {
		return nil, fmt.Errorf("invalid JSON from oracle: %s", cleaned)
	}

	// Валидация: narrative не пустой
	if result.Narrative == "" {
		return nil, fmt.Errorf("oracle returned empty narrative")
	}

	// Валидация: обрезать лишние события
	maxEvents := sections.MaxEvents
	if maxEvents <= 0 {
		maxEvents = 3
	}
	if len(result.NewEvents) > maxEvents {
		log.Printf("[WARN] Oracle returned %d events, trimming to %d", len(result.NewEvents), maxEvents)
		result.NewEvents = result.NewEvents[:maxEvents]
	}

	// Валидация: ensure world_id и scope_id для каждого события
	for i, ev := range result.NewEvents {
		if _, ok := ev["world_id"].(string); !ok || ev["world_id"] == "" {
			result.NewEvents[i]["world_id"] = sections.DefaultWorldID
		}
		if _, ok := ev["scope_id"]; !ok {
			result.NewEvents[i]["scope_id"] = sections.ScopeID
		}
	}

	return &result, nil
}

// Вспомогательные функции
func buildEventClusters(clusters []EventCluster) string {
	if len(clusters) == 0 {
		return "Нет событий за период.\n"
	}
	var lines []string
	for _, c := range clusters {
		lines = append(lines, fmt.Sprintf("[ %s ] %s", c.RelativeTime, c.Description))
	}
	return strings.Join(lines, "\n")
}

func BuildTimeContext(lastEventTime *time.Time, lastMood []string) string {
	var lines []string
	now := time.Now()
	lines = append(lines, "Абсолютное время: "+now.Format("15:04, 02.01.2006"))
	lines = append(lines, "- Сутки: "+getDayPhase(now))
	lines = append(lines, "- Сезон: "+getSeason(now))

	if lastEventTime != nil {
		ago := now.Sub(*lastEventTime)
		agoDesc := humanizeDuration(int64(ago.Milliseconds()))
		lines = append(lines, fmt.Sprintf("- Последнее событие: %s", agoDesc))
	}

	if len(lastMood) > 0 {
		lines = append(lines, fmt.Sprintf("- Атмосфера: %s", strings.Join(lastMood, ", ")))
	}

	return strings.Join(lines, "\n")
}

func humanizeDuration(dt int64) string {
	switch {
	case dt <= 0:
		return "одновременно"
	case dt <= 50:
		return "почти одновременно"
	case dt <= 200:
		return "мгновенно после"
	case dt <= 800:
		return "через мгновение"
	case dt <= 1500:
		return "через секунду"
	case dt <= 3000:
		return "спустя пару секунд"
	case dt <= 10000:
		return "спустя несколько секунд"
	default:
		secs := dt / 1000
		return fmt.Sprintf("спустя %d секунд", secs)
	}
}

func getDayPhase(t time.Time) string {
	h := t.Hour()
	switch {
	case h >= 5 && h < 12:
		return "утро"
	case h >= 12 && h < 18:
		return "день"
	case h >= 18 && h < 22:
		return "вечер"
	default:
		return "ночь"
	}
}

func getSeason(t time.Time) string {
	month := t.Month()
	switch month {
	case time.December, time.January, time.February:
		return "зима"
	case time.March, time.April, time.May:
		return "весна"
	case time.June, time.July, time.August:
		return "лето"
	default:
		return "осень"
	}
}
