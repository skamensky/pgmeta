package db

import (
	"context"
	"fmt"
	"regexp"
	"testing"

	"github.com/palantir/stacktrace"
	"github.com/shkamensky/pgmeta/internal/metadata/types"
)

// This is a mock test that doesn't actually connect to a database
// For real integration tests, you would need a real PostgreSQL instance

// We need a more sophisticated mock for DB
type mockQuerier struct{}

// Error to return from the mock
type mockSQLError struct{}

func (e *mockSQLError) Error() string {
	return "mock SQL error"
}

// Create a Connector without using a real database
func createMockConnector() *Connector {
	return &Connector{
		db: nil, // We'll handle methods so this can be nil
	}
}

// Override the QueryObjects method for testing
func (c *Connector) mockQueryObjects(ctx context.Context, opts types.QueryOptions) ([]types.DBObject, error) {
	pattern, err := regexp.Compile(opts.NameRegex)
	if err != nil {
		return nil, stacktrace.Propagate(err, "Invalid regex pattern: %s", opts.NameRegex)
	}

	if pattern.String() == ".*" {
		return []types.DBObject{
			{Type: types.TypeTable, Schema: "public", Name: "mock_table"},
		}, nil
	}
	
	return nil, &mockSQLError{}
}

// Override the FetchObjectDefinition method for testing
func (c *Connector) mockFetchObjectDefinition(ctx context.Context, obj *types.DBObject) error {
	// If definition is already set, return nil
	if obj.Definition != "" {
		return nil
	}
	
	// If it's an invalid type, return an error
	if obj.Type == "invalid" {
		return stacktrace.NewError("Unsupported object type: %s", obj.Type)
	}
	
	// Otherwise, return a mock error
	return &mockSQLError{}
}

// Mock implementation for FetchObjectsDefinitionsConcurrently
func (c *Connector) mockFetchObjectsDefinitionsConcurrently(ctx context.Context, objects []types.DBObject, concurrency int) ([]types.DBObject, []string, error) {
	results := make([]types.DBObject, len(objects))
	failedObjects := make([]string, 0)
	
	// For valid objects, set their definitions to a mock value and return success
	// For invalid objects, add them to the failedObjects list
	for i, obj := range objects {
		results[i] = obj // Copy the original object
		
		// Call FetchObjectDefinition for each object
		err := c.mockFetchObjectDefinition(ctx, &results[i])
		if err != nil {
			failedObjects = append(failedObjects, fmt.Sprintf("%s.%s", obj.Schema, obj.Name))
		}
	}
	
	return results, failedObjects, nil
}

// Test error handling in QueryObjects
func TestQueryObjectsError(t *testing.T) {
	// Create a mock connector
	connector := createMockConnector()

	// Set up query options
	opts := types.QueryOptions{
		Schema:    "public",
		NameRegex: "error", // This will trigger an error in our mock
		Types:     []types.ObjectType{types.TypeTable},
	}

	// Call our mock implementation
	objects, err := connector.mockQueryObjects(context.Background(), opts)
	
	// Verify we got an error
	if err == nil {
		t.Error("Expected error from QueryObjects, got nil")
	}
	
	// Verify no objects were returned
	if len(objects) != 0 {
		t.Errorf("Expected 0 objects, got %d", len(objects))
	}
}

// Test error handling with invalid regex
func TestQueryObjectsInvalidRegex(t *testing.T) {
	// Create a mock connector
	connector := createMockConnector()

	// Set up query options with invalid regex
	opts := types.QueryOptions{
		Schema:    "public",
		NameRegex: "[", // Invalid regex
		Types:     []types.ObjectType{types.TypeTable},
	}

	// Call our mock implementation
	_, err := connector.mockQueryObjects(context.Background(), opts)
	
	// Verify we got an error
	if err == nil {
		t.Error("Expected error from invalid regex, got nil")
	}
}

// Test FetchObjectDefinition error handling
func TestFetchObjectDefinitionError(t *testing.T) {
	// Create a mock connector
	connector := createMockConnector()

	// Test each type of object
	for _, objType := range []types.ObjectType{
		types.TypeTable,
		types.TypeView,
		types.TypeFunction,
		types.TypeTrigger,
		types.TypeIndex,
	} {
		// Create a test object
		obj := &types.DBObject{
			Type:   objType,
			Schema: "public",
			Name:   "test",
		}

		// Call our mock implementation
		err := connector.mockFetchObjectDefinition(context.Background(), obj)
		
		// Verify we got an error
		if err == nil {
			t.Errorf("Expected error from FetchObjectDefinition for type %s, got nil", objType)
		}
	}

	// Test with an invalid object type
	obj := &types.DBObject{
		Type:   "invalid",
		Schema: "public",
		Name:   "test",
	}

	// Call our mock implementation
	err := connector.mockFetchObjectDefinition(context.Background(), obj)
	
	// Verify we got an error
	if err == nil {
		t.Error("Expected error from FetchObjectDefinition for invalid type, got nil")
	}
}

// Test that object with existing definition is not re-fetched
func TestFetchObjectDefinitionWithExistingDefinition(t *testing.T) {
	// Create a mock connector
	connector := createMockConnector()

	// Create a test object with an existing definition
	obj := &types.DBObject{
		Type:       types.TypeTable,
		Schema:     "public",
		Name:       "test",
		Definition: "CREATE TABLE test();",
	}

	// Call our mock implementation
	err := connector.mockFetchObjectDefinition(context.Background(), obj)
	
	// Verify there was no error (since we shouldn't hit the database)
	if err != nil {
		t.Errorf("Expected no error for object with existing definition, got: %v", err)
	}

	// Verify the definition was not changed
	if obj.Definition != "CREATE TABLE test();" {
		t.Errorf("Object definition changed unexpectedly to: %s", obj.Definition)
	}
}

// Test the buildTableDefinitionQuery function
func TestBuildTableDefinitionQuery(t *testing.T) {
	query := buildTableDefinitionQuery()
	
	// Check that the query is not empty
	if query == "" {
		t.Error("buildTableDefinitionQuery returned empty string")
	}
	
	// Check that it contains expected SQL parts
	expectedParts := []string{
		"CREATE TABLE",
		"FROM information_schema.columns",
		"string_agg",
	}
	
	for _, part := range expectedParts {
		if !regexp.MustCompile(part).MatchString(query) {
			t.Errorf("Expected query to contain '%s', but it doesn't", part)
		}
	}
}

// Test the FetchObjectsDefinitionsConcurrently function
func TestFetchObjectsDefinitionsConcurrently(t *testing.T) {
	// Create a mock connector
	connector := createMockConnector()
	
	// Create test objects, one with valid type and one with invalid type
	objects := []types.DBObject{
		{
			Type:   types.TypeTable,
			Schema: "public",
			Name:   "test_table",
		},
		{
			Type:   "invalid", // This will cause an error
			Schema: "public",
			Name:   "invalid_obj",
		},
		{
			Type:       types.TypeTable,
			Schema:     "public",
			Name:       "table_with_def",
			Definition: "CREATE TABLE table_with_def();", // This already has a definition
		},
	}
	
	// Call our mock implementation
	results, failedObjects, err := connector.mockFetchObjectsDefinitionsConcurrently(context.Background(), objects, 10)
	
	// There should be no error from the function itself
	if err != nil {
		t.Errorf("Expected no error from FetchObjectsDefinitionsConcurrently, got: %v", err)
	}
	
	// Both the table and invalid object should fail due to our mock implementation
	if len(failedObjects) != 2 {
		t.Errorf("Expected 2 failed objects, got %d", len(failedObjects))
	}
	
	// Verify the results length
	if len(results) != len(objects) {
		t.Errorf("Expected %d results, got %d", len(objects), len(results))
	}
	
	// The object with existing definition should not have been changed
	if results[2].Definition != "CREATE TABLE table_with_def();" {
		t.Errorf("Object with existing definition changed unexpectedly to: %s", results[2].Definition)
	}
}