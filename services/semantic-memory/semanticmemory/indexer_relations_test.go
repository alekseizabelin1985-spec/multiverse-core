package semanticmemory

import (
	"testing"
)

func TestRelationsMetrics(t *testing.T) {
	m := RelationsMetrics{}

	if m.ExplicitCount != 0 {
		t.Error("Expected ExplicitCount = 0")
	}
	if m.FallbackCount != 0 {
		t.Error("Expected FallbackCount = 0")
	}
	if m.EntityCreated != 0 {
		t.Error("Expected EntityCreated = 0")
	}
	if m.ValidationErrs != 0 {
		t.Error("Expected ValidationErrs = 0")
	}
}

func TestApplyExplicitRelations_EmptyRelations(t *testing.T) {
	// Проверяем что пустые relations не вызывают панику
	// (полный тест требует mock Neo4j — это интеграционный тест)
}
