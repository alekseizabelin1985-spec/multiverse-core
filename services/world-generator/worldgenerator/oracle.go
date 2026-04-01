// Package worldgenerator handles communication with Ascension Oracle.
package worldgenerator

import (
	"context"
	"fmt"
	"strings"

	"multiverse-core.io/shared/oracle"
)

// generateWorldConcept ЭТАП A: генерация концепции мира
func (wg *WorldGenerator) generateWorldConcept(ctx context.Context, req *WorldGenerationRequest) (*WorldConcept, error) {
	systemPrompt, userPrompt := buildConceptPrompts(req)

	var concept WorldConcept
	err := wg.oracle.CallAndUnmarshal(ctx, func() (string, error) {
		return wg.oracle.CallStructuredJSON(ctx, systemPrompt, userPrompt)
	}, &concept)
	if err != nil {
		return nil, fmt.Errorf("concept generation failed: %w", err)
	}

	// Если scale не задан в концепции — унаследовать из запроса
	if concept.Scale == "" {
		concept.Scale = req.getScale()
	}

	return &concept, nil
}

// generateWorldDetails ЭТАП B: детализация географии и онтологии на основе концепции
func (wg *WorldGenerator) generateWorldDetails(ctx context.Context, worldID string, concept *WorldConcept, scale string) (*WorldGeography, error) {
	systemPrompt, userPrompt := buildDetailsPrompts(concept, scale)

	var geography WorldGeography
	err := wg.oracle.CallAndUnmarshal(ctx, func() (string, error) {
		return wg.oracle.CallStructuredJSON(ctx, systemPrompt, userPrompt)
	}, &geography)
	if err != nil {
		return nil, fmt.Errorf("world details generation failed: %w", err)
	}

	return &geography, nil
}

// buildConceptPrompts формирует system и user промпты для этапа A (концепция)
func buildConceptPrompts(req *WorldGenerationRequest) (systemPrompt, userPrompt string) {
	systemPrompt = `Ты — Демиург, создатель миров. Твоя задача — создать уникальную концепцию мира.
Отвечай строго в формате JSON без пояснений.

Формат ответа:
{
  "core": "описание ядра мира (2-3 предложения)",
  "theme": "основная тема мира",
  "era": "эпоха или временной период",
  "unique_traits": ["черта 1", "черта 2", "черта 3"],
  "scale": "small|medium|large"
}`

	if req.Mode == "contextual" && req.UserContext != nil {
		// Контекстный режим — включаем описание пользователя
		userPrompt = fmt.Sprintf(`Создай концепцию мира на основе следующего описания.

Семя мира: "%s"
Описание: %s
Тема: %s
Ключевые элементы: %s
Масштаб: %s
Ограничения (чего НЕ должно быть): %s

Концепция должна уважать все указанные элементы и ограничения, но может творчески дополнять и расширять описание.`,
			req.Seed,
			req.UserContext.Description,
			defaultIfEmpty(req.UserContext.Theme, "на усмотрение Демиурга"),
			strings.Join(req.UserContext.KeyElements, ", "),
			defaultIfEmpty(req.UserContext.Scale, "medium"),
			strings.Join(req.UserContext.Restrictions, ", "),
		)
	} else {
		// Случайный режим
		userPrompt = fmt.Sprintf(`Создай полностью оригинальную и неожиданную концепцию мира.

Семя мира: "%s"

Мир может быть любой тематики — фэнтези, научная фантастика, мифология, стимпанк, пост-апокалипсис, культивация, или что-то совершенно необычное. Удиви.
Масштаб: medium.`, req.Seed)
	}

	return systemPrompt, userPrompt
}

// buildDetailsPrompts формирует system и user промпты для этапа B (детализация)
func buildDetailsPrompts(concept *WorldConcept, scale string) (systemPrompt, userPrompt string) {
	minR, maxR, minW, maxW, minC, maxC := scaleParams(scale)

	systemPrompt = fmt.Sprintf(`Ты — Демиург, детализирующий мир.

Концепция мира:
- Ядро: %s
- Тема: %s
- Эпоха: %s
- Уникальные черты: %s

Все детали должны быть внутренне согласованы с этой концепцией. Названия, биомы, города — всё должно отражать тему и эпоху мира.

Отвечай строго в формате JSON без пояснений.`,
		concept.Core,
		concept.Theme,
		concept.Era,
		strings.Join(concept.UniqueTraits, "; "),
	)

	userPrompt = fmt.Sprintf(`Сгенерируй полную детализацию мира.

Требования:
1. Онтология (система силы/прогрессии, соответствующая теме мира):
   - system: тип системы (cultivation, magic, technology, divine, nature и т.д.)
   - carriers: носители силы (2-4 шт.)
   - paths: пути развития (3-5 шт.)
   - forbidden: запреты/табу (2-3 шт.)
   - hierarchy: уровни прогрессии (4-7 шт.)

2. География:
   - %d-%d регионов с уникальными биомами, координатами и размером
   - %d-%d водных объектов (реки, моря, озёра) с координатами и размером
   - %d-%d городов с населением, типом (major/minor) и привязкой к региону

3. Мифология: краткий основополагающий миф мира (3-5 предложений)

Формат JSON:
{
  "core": "string (повторить ядро мира)",
  "ontology": {
    "system": "string",
    "carriers": ["string"],
    "paths": ["string"],
    "forbidden": ["string"],
    "hierarchy": ["string"]
  },
  "geography": {
    "regions": [{"name": "string", "biome": "string", "coordinates": {"x": 0.0, "y": 0.0}, "size": 0.0}],
    "water_bodies": [{"name": "string", "type": "river|sea|lake", "coordinates": {"x": 0.0, "y": 0.0}, "size": 0.0}],
    "cities": [{"name": "string", "population": 0, "type": "major|minor", "location": {"region": "string", "coordinates": {"x": 0.0, "y": 0.0}}}]
  },
  "mythology": "string"
}`, minR, maxR, minW, maxW, minC, maxC)

	return systemPrompt, userPrompt
}

// CallOracle отправляет промпт в Ascension Oracle и возвращает ответ (для обратной совместимости).
func CallOracle(ctx context.Context, prompt string) (string, error) {
	client := oracle.NewClient()
	content, err := client.CallAndLog(ctx, prompt)
	if err != nil {
		return "", fmt.Errorf("failed to connect to oracle: %w", err)
	}
	if content == "" {
		return "", fmt.Errorf("oracle returned empty content")
	}
	return content, nil
}
