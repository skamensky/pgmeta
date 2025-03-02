package types

import "testing"

func TestIsValidType(t *testing.T) {
	// Test all valid types
	validTypes := []ObjectType{
		TypeTable,
		TypeView,
		TypeFunction,
		TypeAggregate,
		TypeTrigger,
		TypeIndex,
		TypeConstraint,
	}

	for _, typeName := range validTypes {
		if !IsValidType(typeName) {
			t.Errorf("Expected %s to be a valid type", typeName)
		}
	}

	// Test an invalid type
	if IsValidType("invalid") {
		t.Error("Expected 'invalid' to be an invalid type")
	}
}

func TestContainsAny(t *testing.T) {
	// Test with empty slice (should return true)
	if !ContainsAny(nil, TypeTable) {
		t.Error("ContainsAny with nil slice should return true")
	}

	if !ContainsAny([]ObjectType{}, TypeTable) {
		t.Error("ContainsAny with empty slice should return true")
	}

	// Test with matching type
	if !ContainsAny([]ObjectType{TypeTable, TypeView}, TypeTable) {
		t.Error("ContainsAny should return true when slice contains the element")
	}

	// Test with non-matching type
	if ContainsAny([]ObjectType{TypeTable, TypeView}, TypeFunction) {
		t.Error("ContainsAny should return false when slice doesn't contain the element")
	}

	// Test with multiple types to check
	if !ContainsAny([]ObjectType{TypeTable, TypeView}, TypeFunction, TypeTable) {
		t.Error("ContainsAny should return true when slice contains any of the elements")
	}
}
