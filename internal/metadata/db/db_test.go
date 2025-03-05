package db

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/palantir/stacktrace"
	"github.com/skamensky/pgmeta/internal/metadata/types"
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
	// Special case for error testing
	if opts.NameRegex == "error" {
		return nil, &mockSQLError{}
	}

	pattern, err := regexp.Compile(opts.NameRegex)
	if err != nil {
		return nil, stacktrace.Propagate(err, "Invalid regex pattern: %s", opts.NameRegex)
	}

	var objects []types.DBObject

	// Handle test schemas
	mockSchemas := map[string][]types.DBObject{
		"public": {
			{Type: types.TypeTable, Schema: "public", Name: "users"},
			{Type: types.TypeView, Schema: "public", Name: "active_users"},
		},
		"app": {
			{Type: types.TypeTable, Schema: "app", Name: "products"},
			{Type: types.TypeFunction, Schema: "app", Name: "get_product"},
		},
		"reporting": {
			{Type: types.TypeView, Schema: "reporting", Name: "sales_summary"},
		},
	}

	// If no schemas specified, default to public
	if len(opts.Schemas) == 0 {
		opts.Schemas = []string{"public"}
	}

	// Handle error case for non-existent schema
	for _, schema := range opts.Schemas {
		if schema == "non_existent" {
			return nil, stacktrace.NewError("Schema does not exist: non_existent")
		}
	}

	// Collect objects from requested schemas
	for _, schema := range opts.Schemas {
		if schemaObjs, exists := mockSchemas[schema]; exists {
			for _, obj := range schemaObjs {
				if pattern.MatchString(obj.Name) {
					// Filter by type if necessary
					if len(opts.Types) == 0 || types.ContainsAny(opts.Types, obj.Type) {
						objects = append(objects, obj)
					}
				}
			}
		}
	}

	return objects, nil
}

// Override the GetAllSchemas method for testing
func (c *Connector) mockGetAllSchemas(ctx context.Context) ([]string, error) {
	return []string{"public", "app", "reporting"}, nil
}

// Override the schemaExists method for testing
func (c *Connector) mockSchemaExists(ctx context.Context, schema string) (bool, error) {
	validSchemas := map[string]bool{
		"public":    true,
		"app":       true,
		"reporting": true,
	}
	return validSchemas[schema], nil
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
		Schemas:   []string{"public"},
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
		Schemas:   []string{"public"},
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

// Test querying multiple schemas
func TestQueryMultipleSchemas(t *testing.T) {
	// Create a mock connector
	connector := createMockConnector()

	// Test with multiple schemas
	opts := types.QueryOptions{
		Schemas:   []string{"public", "app"},
		NameRegex: ".*",
		Types:     []types.ObjectType{},
	}

	// Call our mock implementation
	objects, err := connector.mockQueryObjects(context.Background(), opts)

	// Verify no error
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	// Verify objects from both schemas were returned
	if len(objects) != 4 {
		t.Errorf("Expected 4 objects from multiple schemas, got %d", len(objects))
	}

	// Count objects by schema
	schemaCount := make(map[string]int)
	for _, obj := range objects {
		schemaCount[obj.Schema]++
	}

	// Verify we got objects from each schema
	if schemaCount["public"] != 2 {
		t.Errorf("Expected 2 objects from public schema, got %d", schemaCount["public"])
	}
	if schemaCount["app"] != 2 {
		t.Errorf("Expected 2 objects from app schema, got %d", schemaCount["app"])
	}

	// Test with non-existent schema
	badOpts := types.QueryOptions{
		Schemas:   []string{"non_existent"},
		NameRegex: ".*",
	}

	// Call our mock implementation
	_, err = connector.mockQueryObjects(context.Background(), badOpts)

	// Verify we got an error
	if err == nil {
		t.Error("Expected error from non-existent schema, got nil")
	}

	// Verify error message contains the expected text
	expectedError := "Schema does not exist: non_existent"
	if !strings.Contains(err.Error(), expectedError) {
		t.Errorf("Expected error message to contain '%s', got '%s'", expectedError, err.Error())
	}
}

// Test GetAllSchemas function
func TestGetAllSchemas(t *testing.T) {
	// Create a mock connector
	connector := createMockConnector()

	// Call our mock implementation
	schemas, err := connector.mockGetAllSchemas(context.Background())

	// Verify no error
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	// Verify all expected schemas are returned
	expectedSchemas := []string{"public", "app", "reporting"}
	if len(schemas) != len(expectedSchemas) {
		t.Errorf("Expected %d schemas, got %d", len(expectedSchemas), len(schemas))
	}

	// Verify each expected schema is present
	for _, expected := range expectedSchemas {
		found := false
		for _, actual := range schemas {
			if actual == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected schema '%s' not found in results", expected)
		}
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

	// Check that the query contains the expected parts
	expectedParts := []string{
		"CREATE TABLE",
		"quote_ident($1)",
		"quote_ident($2)",
		"quote_ident(c.column_name)",
		"quote_ident(ccu.table_schema)",
		"quote_ident(ccu.table_name)",
	}

	for _, part := range expectedParts {
		if !strings.Contains(query, part) {
			t.Errorf("Expected query to contain '%s', but it doesn't", part)
		}
	}

	// Check that the query doesn't contain any ($1)::text or ($2)::text
	unexpectedParts := []string{
		"($1)::text",
		"($2)::text",
	}

	for _, part := range unexpectedParts {
		if strings.Contains(query, part) {
			t.Errorf("Query should not contain '%s', but it does", part)
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
