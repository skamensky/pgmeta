package metadata

import (
	"testing"

	"github.com/shkamensky/pgmeta/internal/metadata/types"
)

// Test the IsValidType function
func TestIsValidType(t *testing.T) {
	// Test valid types
	validTypes := []types.ObjectType{
		types.TypeTable,
		types.TypeView,
		types.TypeFunction,
		types.TypeTrigger,
		types.TypeIndex,
		types.TypeConstraint,
	}

	for _, typeName := range validTypes {
		if !IsValidType(typeName) {
			t.Errorf("IsValidType(%s) returned false, expected true", typeName)
		}
	}

	// Test invalid type
	if IsValidType("invalid") {
		t.Error("IsValidType(invalid) returned true, expected false")
	}
}

// Note: More comprehensive tests for Fetcher would require a real database connection
// or a more sophisticated mock. The components that make up the Fetcher are tested
// in their respective packages.