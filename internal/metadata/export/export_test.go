package export

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/shkamensky/pgmeta/internal/metadata/types"
)

// Define our own interface for the connector
type dbConnector interface {
	FetchObjectDefinition(ctx context.Context, obj *types.DBObject) error
}

// Mock connector for testing
type mockConnector struct {
	shouldFail bool
	mu         sync.Mutex // To make the mock thread-safe
}

func (m *mockConnector) FetchObjectDefinition(ctx context.Context, obj *types.DBObject) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if m.shouldFail {
		return &mockError{}
	}
	
	// If the definition is already set, do nothing
	if obj.Definition != "" {
		return nil
	}
	
	// Otherwise, set a mock definition based on the object type
	switch obj.Type {
	case types.TypeTable:
		obj.Definition = "CREATE TABLE " + obj.Schema + "." + obj.Name + " (id integer);"
	case types.TypeView:
		obj.Definition = "CREATE VIEW " + obj.Schema + "." + obj.Name + " AS SELECT 1;"
	case types.TypeFunction:
		obj.Definition = "CREATE FUNCTION " + obj.Schema + "." + obj.Name + "() RETURNS void AS $$ BEGIN END; $$ LANGUAGE plpgsql;"
	case types.TypeTrigger:
		obj.Definition = "CREATE TRIGGER " + obj.Name + " AFTER INSERT ON " + obj.Schema + "." + obj.TableName + " FOR EACH ROW EXECUTE PROCEDURE trigger_func();"
	case types.TypeIndex:
		obj.Definition = "CREATE INDEX " + obj.Name + " ON " + obj.Schema + "." + obj.TableName + " (id);"
	case types.TypeConstraint:
		obj.Definition = "ALTER TABLE " + obj.Schema + "." + obj.TableName + " ADD CONSTRAINT " + obj.Name + " PRIMARY KEY (id);"
	default:
		obj.Definition = "MOCK DEFINITION for " + string(obj.Type) + " " + obj.Name;
	}
	
	return nil
}

type mockError struct{}

func (m *mockError) Error() string {
	return "mock error"
}

// Override the New function for testing with our mockConnector
func NewWithMock(connector dbConnector, outputDir string) *Exporter {
	return &Exporter{
		connector:      connector,
		outputDir:      outputDir,
		concurrency:    10, // Smaller concurrency for tests
	}
}

// NewWithMockAndConcurrency creates a test exporter with specified concurrency
func NewWithMockAndConcurrency(connector dbConnector, outputDir string, concurrency int) *Exporter {
	return &Exporter{
		connector:      connector,
		outputDir:      outputDir,
		concurrency:    concurrency,
	}
}

func TestExportObjects(t *testing.T) {
	// Create a temporary directory for output
	tmpDir, err := ioutil.TempDir("", "pgmeta-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create test objects
	objects := []types.DBObject{
		{
			Type:   types.TypeTable,
			Schema: "public",
			Name:   "users",
		},
		{
			Type:      types.TypeIndex,
			Schema:    "public",
			Name:      "users_idx",
			TableName: "users",
		},
		{
			Type:      types.TypeConstraint,
			Schema:    "public",
			Name:      "users_pk",
			TableName: "users",
			Definition: "PRIMARY KEY (id)",  // Pre-defined
		},
		{
			Type:      types.TypeTrigger,
			Schema:    "public",
			Name:      "users_audit",
			TableName: "users",
		},
		{
			Type:   types.TypeFunction,
			Schema: "public",
			Name:   "get_user",
		},
		{
			Type:   types.TypeView,
			Schema: "public",
			Name:   "active_users",
		},
	}

	// Create exporter with mock connector
	connector := &mockConnector{shouldFail: false}
	exporter := NewWithMock(connector, tmpDir)

	// Export objects
	err = exporter.ExportObjects(context.Background(), objects)
	if err != nil {
		t.Fatalf("ExportObjects failed: %v", err)
	}

	// Verify directories were created
	expectedDirs := []string{
		filepath.Join(tmpDir, "tables", "users"),
		filepath.Join(tmpDir, "tables", "users", "indexes"),
		filepath.Join(tmpDir, "tables", "users", "constraints"),
		filepath.Join(tmpDir, "tables", "users", "triggers"),
		filepath.Join(tmpDir, "functions"),
		filepath.Join(tmpDir, "views"),
	}

	for _, dir := range expectedDirs {
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			t.Errorf("Expected directory was not created: %s", dir)
		}
	}

	// Verify files were created
	expectedFiles := []string{
		filepath.Join(tmpDir, "tables", "users", "table.sql"),
		filepath.Join(tmpDir, "tables", "users", "indexes", "users_idx.sql"),
		filepath.Join(tmpDir, "tables", "users", "constraints", "users_pk.sql"),
		filepath.Join(tmpDir, "tables", "users", "triggers", "users_audit.sql"),
		filepath.Join(tmpDir, "functions", "public.get_user.sql"),
		filepath.Join(tmpDir, "views", "public.active_users.sql"),
	}

	for _, file := range expectedFiles {
		if _, err := os.Stat(file); os.IsNotExist(err) {
			t.Errorf("Expected file was not created: %s", file)
		}
	}
}

func TestExportObjectsWithFetchError(t *testing.T) {
	// Create a temporary directory for output
	tmpDir, err := ioutil.TempDir("", "pgmeta-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create test objects
	objects := []types.DBObject{
		{
			Type:   types.TypeTable,
			Schema: "public",
			Name:   "users",
		},
	}

	// Create exporter with failing mock connector
	connector := &mockConnector{shouldFail: true}
	exporter := NewWithMock(connector, tmpDir)

	// Export objects, should fail
	err = exporter.ExportObjects(context.Background(), objects)
	if err == nil {
		t.Error("Expected ExportObjects to fail, but it succeeded")
	}

	// Verify no files were created
	entries, _ := ioutil.ReadDir(tmpDir)
	if len(entries) > 0 {
		t.Errorf("Expected no files to be created, but found %d entries", len(entries))
	}
}

func TestExportObjectWithNoTableName(t *testing.T) {
	// Create a temporary directory for output
	tmpDir, err := ioutil.TempDir("", "pgmeta-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create test objects
	objects := []types.DBObject{
		{
			Type:   types.TypeTable,
			Schema: "public",
			Name:   "users",
		},
		{
			Type:   types.TypeTrigger,  // Missing TableName
			Schema: "public",
			Name:   "orphan_trigger",
		},
	}

	// Create exporter with mock connector
	connector := &mockConnector{shouldFail: false}
	exporter := NewWithMock(connector, tmpDir)

	// Export objects
	err = exporter.ExportObjects(context.Background(), objects)
	if err != nil {
		t.Fatalf("ExportObjects failed: %v", err)
	}

	// The orphan trigger should be exported as a standalone object
	triggerFile := filepath.Join(tmpDir, "triggers", "public.orphan_trigger.sql")
	if _, err := os.Stat(triggerFile); os.IsNotExist(err) {
		t.Errorf("Expected orphan trigger file was not created: %s", triggerFile)
	}
}

func TestConcurrentExport(t *testing.T) {
	// Create a temporary directory for output
	tmpDir, err := ioutil.TempDir("", "pgmeta-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a larger set of test objects to test concurrency
	objects := make([]types.DBObject, 0)
	
	// Add multiple tables with multiple objects each
	for i := 1; i <= 10; i++ {
		tableName := fmt.Sprintf("table_%d", i)
		// Add table
		objects = append(objects, types.DBObject{
			Type:   types.TypeTable,
			Schema: "public",
			Name:   tableName,
		})
		
		// Add multiple indexes per table
		for j := 1; j <= 5; j++ {
			objects = append(objects, types.DBObject{
				Type:      types.TypeIndex,
				Schema:    "public",
				Name:      fmt.Sprintf("%s_idx_%d", tableName, j),
				TableName: tableName,
			})
		}
		
		// Add multiple constraints per table
		for j := 1; j <= 3; j++ {
			objects = append(objects, types.DBObject{
				Type:      types.TypeConstraint,
				Schema:    "public",
				Name:      fmt.Sprintf("%s_con_%d", tableName, j),
				TableName: tableName,
			})
		}
		
		// Add triggers per table
		for j := 1; j <= 2; j++ {
			objects = append(objects, types.DBObject{
				Type:      types.TypeTrigger,
				Schema:    "public",
				Name:      fmt.Sprintf("%s_trg_%d", tableName, j),
				TableName: tableName,
			})
		}
	}
	
	// Add standalone objects
	for i := 1; i <= 20; i++ {
		objects = append(objects, types.DBObject{
			Type:   types.TypeFunction,
			Schema: "public",
			Name:   fmt.Sprintf("function_%d", i),
		})
	}
	
	for i := 1; i <= 15; i++ {
		objects = append(objects, types.DBObject{
			Type:   types.TypeView,
			Schema: "public",
			Name:   fmt.Sprintf("view_%d", i),
		})
	}

	// Create exporter with mock connector and higher concurrency
	connector := &mockConnector{shouldFail: false}
	exporter := NewWithMockAndConcurrency(connector, tmpDir, 20)

	// Export objects
	start := time.Now()
	err = exporter.ExportObjects(context.Background(), objects)
	duration := time.Since(start)
	
	if err != nil {
		t.Fatalf("ExportObjects failed: %v", err)
	}
	
	t.Logf("Exported %d objects in %v", len(objects), duration)

	// Verify the number of files created matches the objects
	fileCount := countFiles(t, tmpDir)
	if fileCount != len(objects) {
		t.Errorf("Expected %d files, but found %d", len(objects), fileCount)
	}
	
	// Try with single thread for comparison
	// Create a new temp dir
	singleThreadDir, err := ioutil.TempDir("", "pgmeta-single")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(singleThreadDir)
	
	// Create single-threaded exporter
	singleThreadExporter := NewWithMockAndConcurrency(connector, singleThreadDir, 1)
	
	// Export objects with single thread
	startSingle := time.Now()
	err = singleThreadExporter.ExportObjects(context.Background(), objects)
	durationSingle := time.Since(startSingle)
	
	if err != nil {
		t.Fatalf("Single-threaded ExportObjects failed: %v", err)
	}
	
	t.Logf("Single-threaded: Exported %d objects in %v", len(objects), durationSingle)
	
	// Verify files created matches objects
	singleFileCount := countFiles(t, singleThreadDir)
	if singleFileCount != len(objects) {
		t.Errorf("Single-threaded: Expected %d files, but found %d", len(objects), singleFileCount)
	}
}

// Helper function to count files in a directory recursively
func countFiles(t *testing.T, dir string) int {
	count := 0
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			count++
		}
		return nil
	})
	
	if err != nil {
		t.Fatalf("Error counting files: %v", err)
	}
	
	return count
}