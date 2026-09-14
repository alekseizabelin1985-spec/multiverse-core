package entity

import "slices"

// AttributeNames are the rows the table holds for a type, sorted, so that a test
// can compare the set with data-model.md §3 in both directions.
func AttributeNames(entityType string) []string {
	names := make([]string, 0, len(attributeTable[entityType]))
	for name := range attributeTable[entityType] {
		names = append(names, name)
	}
	slices.Sort(names)
	return names
}

// AttributeTypes are the types the table has rows for, sorted.
func AttributeTypes() []string {
	types := make([]string, 0, len(attributeTable))
	for typ := range attributeTable {
		types = append(types, typ)
	}
	slices.Sort(types)
	return types
}
