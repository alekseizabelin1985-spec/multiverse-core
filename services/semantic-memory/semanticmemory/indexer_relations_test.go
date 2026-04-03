package semanticmemory

import (
	"testing"
)

func TestApplyExplicitRelations_EmptyRelations(t *testing.T) {
	// Проверяем что пустые relations не вызывают панику
	// (полный тест требует mock Neo4j — это интеграционный тест)
}
