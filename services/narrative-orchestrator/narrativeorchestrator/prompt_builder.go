// services/narrativeorchestrator/prompt_builder.go

package narrativeorchestrator

import (
	"fmt"
	"strings"
	"time"
)

// PromptSections — структурированный ввод для билдера промтов.
type PromptSections struct {
	// FACTS: данные о мире (меняются каждый вызов)
	WorldFacts   string   // Канон мира, законы, история
	EntityStates string   // Текущее состояние сущностей в области
	Canon        []string // Неизменные факты мира (из KnowledgeBase)

	// SITUATION: что происходит сейчас (меняется каждый вызов)
	ScopeID       string
	ScopeType     string
	WorldID       string
	TimeContext   string
	EventClusters []EventCluster
	TriggerEvent  string
	LastMood      []string

	// CONSTRAINTS: параметры из конфига GM
	MaxEvents      int    // default 3
	DefaultSource  string // e.g. "narrative-orchestrator"
	DefaultWorldID string // e.g. "pain-realm"
}

// BuildStructuredPrompt строит system и user промты из PromptSections.
// System prompt содержит только кэшируемые части (роль, правила, схема).
// User prompt содержит только изменяемые данные (факты, ситуация, задача).
func BuildStructuredPrompt(s PromptSections) (systemPrompt, userPrompt string) {
	maxEvents := s.MaxEvents
	if maxEvents <= 0 {
		maxEvents = 3
	}
	defaultWorldID := s.DefaultWorldID
	if defaultWorldID == "" {
		defaultWorldID = "unknown-world"
	}
	defaultSource := s.DefaultSource
	if defaultSource == "" {
		defaultSource = "narrative-orchestrator"
	}

	// ── System prompt (кэшируемый) ──────────────────────────────────────────
	var sys strings.Builder

	sys.WriteString("<role>\n")
	sys.WriteString("Ты — Повествователь Мира. Наблюдаешь за событиями и создаёшь иммерсивное повествование.\n")
	sys.WriteString("Ты НЕ принимаешь решений за персонажей.\n")
	sys.WriteString("</role>\n")

	if len(s.Canon) > 0 {
		sys.WriteString("\n<canon>\n")
		for _, fact := range s.Canon {
			sys.WriteString("• ")
			sys.WriteString(fact)
			sys.WriteString("\n")
		}
		sys.WriteString("</canon>\n")
	}

	sys.WriteString("\n<rules>\n")
	sys.WriteString(fmt.Sprintf("• МАКСИМУМ %d событий в new_events.\n", maxEvents))
	sys.WriteString("• События должны быть СЕМАНТИЧЕСКИМИ (звук за дверью, появление сущности), а не атомарными действиями.\n")
	sys.WriteString("• Каждое событие ОБЯЗАТЕЛЬНО содержит: event_type, timestamp, source, world_id, scope_id, payload.\n")
	sys.WriteString("• event_type — snake_case (например: \"environment.sound\", \"player.reacted\").\n")
	sys.WriteString("• timestamp — ISO 8601 UTC (оканчивается на Z).\n")
	sys.WriteString(fmt.Sprintf("• world_id: \"%s\" (ОБЯЗАТЕЛЬНО, не пустая строка).\n", defaultWorldID))
	sys.WriteString("• scope_id присутствует как свойство (значение может быть пустой строкой \"\").\n")
	sys.WriteString("• payload — объект с произвольными полями, релевантными событию. Всегда валидный объект {}.\n")
	sys.WriteString("• mood — массив строк (может быть пустым []).\n")
	sys.WriteString("• Ответ должен начинаться с { и заканчиваться }. Без комментариев //, многоточий ..., кавычек-ёлочек «».\n")
	sys.WriteString("</rules>\n")

	sys.WriteString("\n<schema>\n")
	sys.WriteString("{\n")
	sys.WriteString("  \"narrative\": \"1–3 предложения повествования\",\n")
	sys.WriteString("  \"mood\": [\"настроение1\", \"настроение2\",\"\"],\n")
	sys.WriteString("  \"new_events\": [\n")
	sys.WriteString("    {\n")
	sys.WriteString("      \"event_type\": \"тип.события\",\n")
	sys.WriteString("      \"timestamp\": \"2026-01-01T00:00:00Z\",\n")
	sys.WriteString(fmt.Sprintf("      \"source\": \"%s\",\n", defaultSource))
	sys.WriteString(fmt.Sprintf("      \"world_id\": \"%s\",\n", defaultWorldID))
	sys.WriteString("      \"scope_id\": \"область\",\n")
	sys.WriteString("      \"payload\": {\"description\": \"краткое описание\", \"любые_поля_тип\": \"в зависимости от контекста\"}\n")
	sys.WriteString("    }\n")
	sys.WriteString("  ]\n")
	sys.WriteString("}\n")
	sys.WriteString("</schema>\n")

	systemPrompt = strings.TrimSpace(sys.String())

	// ── User prompt (изменяемый) ────────────────────────────────────────────
	var usr strings.Builder

	usr.WriteString("<facts>\n")
	usr.WriteString(fmt.Sprintf("<scope id=%q type=%q world=%q/>\n", s.ScopeID, s.ScopeType, s.WorldID))
	if s.WorldFacts != "" {
		usr.WriteString("<world>\n")
		usr.WriteString(s.WorldFacts)
		usr.WriteString("\n</world>\n")
	}
	if s.EntityStates != "" {
		usr.WriteString("<entities>\n")
		usr.WriteString(s.EntityStates)
		usr.WriteString("\n</entities>\n")
	}
	usr.WriteString("</facts>\n")

	usr.WriteString("\n<situation>\n")
	if s.TimeContext != "" {
		usr.WriteString("<time>\n")
		usr.WriteString(s.TimeContext)
		usr.WriteString("\n</time>\n")
	}
	if len(s.LastMood) > 0 {
		usr.WriteString(fmt.Sprintf("<mood>%s</mood>\n", strings.Join(s.LastMood, ", ")))
	}
	usr.WriteString("<events>\n")
	usr.WriteString(buildEventClusters(s.EventClusters))
	usr.WriteString("</events>\n")
	if s.TriggerEvent != "" {
		usr.WriteString("<trigger>\n")
		usr.WriteString(s.TriggerEvent)
		usr.WriteString("\n</trigger>\n")
	}
	usr.WriteString("</situation>\n")

	usr.WriteString("\n<task>Продолжи повествование: что логично происходит дальше в этой области?\n\n")
	usr.WriteString("— Учитывай факты, характеры, обстановку.\n")
	usr.WriteString("— Даже если событий мало — мир живёт.\n")
	usr.WriteString("— Используй стилевые модификаторы: «внезапно», «плавно», «тревожно».</task>\n")

	userPrompt = strings.TrimSpace(usr.String())

	fmt.Println("system:", systemPrompt)
	fmt.Println("user:", userPrompt)
	return
}

// MigratePromptInput конвертирует старый PromptInput в новый PromptSections.
func MigratePromptInput(old PromptInput) PromptSections {
	return PromptSections{
		WorldFacts:     old.WorldContext,
		EntityStates:   old.EntitiesContext,
		ScopeID:        old.ScopeID,
		ScopeType:      old.ScopeType,
		TimeContext:    old.TimeContext,
		EventClusters:  old.EventClusters,
		TriggerEvent:   old.TriggerEvent,
		MaxEvents:      3,
		DefaultSource:  "narrative-orchestrator",
		DefaultWorldID: "unknown-world",
	}
}

// BuildTimeContextStructured — улучшенная версия BuildTimeContext с поддержкой игрового времени.
// gameTimeMs — опциональное игровое время в миллисекундах (nil = не используется).
func BuildTimeContextStructured(lastEventTime *time.Time, lastMood []string, gameTimeMs *int64) string {
	var lines []string
	now := time.Now()
	lines = append(lines, "Реальное время: "+now.Format("15:04, 02.01.2006"))
	lines = append(lines, "- Сутки: "+getDayPhase(now))
	lines = append(lines, "- Сезон: "+getSeason(now))

	if gameTimeMs != nil {
		gameTime := time.UnixMilli(*gameTimeMs).UTC()
		lines = append(lines, "Игровое время: "+gameTime.Format("15:04, 02.01.2006 UTC"))
	}

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
