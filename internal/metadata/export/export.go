package export

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/palantir/stacktrace"
	"github.com/shkamensky/pgmeta/internal/log"
	"github.com/shkamensky/pgmeta/internal/metadata/db"
	"github.com/shkamensky/pgmeta/internal/metadata/types"
)

// Define the interface we need from the connector
type DBConnector interface {
	FetchObjectDefinition(ctx context.Context, obj *types.DBObject) error
}

// Exporter handles exporting database objects to files
type Exporter struct {
	connector DBConnector
	outputDir string
}

// New creates a new exporter
func New(connector *db.Connector, outputDir string) *Exporter {
	return &Exporter{
		connector: connector,
		outputDir: outputDir,
	}
}

// ExportObjects exports database objects to files
func (e *Exporter) ExportObjects(ctx context.Context, objects []types.DBObject) error {
	// Track if any objects failed
	hasFailures := false
	failedObjects := make([]string, 0)

	// Group objects by their tables
	tableObjects := make(map[string][]types.DBObject)
	standalone := make([]types.DBObject, 0)

	for _, obj := range objects {
		// Fetch definition for all objects
		if err := e.connector.FetchObjectDefinition(ctx, &obj); err != nil {
			hasFailures = true
			failedObjects = append(failedObjects, fmt.Sprintf("%s.%s", obj.Schema, obj.Name))
			log.Warn("Failed to fetch definition for %s.%s: %v", obj.Schema, obj.Name, err)
			continue
		}

		switch obj.Type {
		case types.TypeTable:
			tableObjects[obj.Name] = append(tableObjects[obj.Name], obj)
		case types.TypeTrigger, types.TypeIndex, types.TypeConstraint:
			// Use the TableName field we populated during query
			if obj.TableName != "" {
				tableObjects[obj.TableName] = append(tableObjects[obj.TableName], obj)
			} else {
				log.Warn("%s %s has no associated table name", obj.Type, obj.Name)
				standalone = append(standalone, obj)
			}
		default:
			standalone = append(standalone, obj)
		}
	}

	// Don't save anything if there were failures
	if hasFailures {
		return stacktrace.NewError("Failed to fetch definitions for objects: %v", failedObjects)
	}

	log.Info("Exporting %d objects to %s", len(objects), e.outputDir)

	// Ensure output directory exists
	if err := os.MkdirAll(e.outputDir, 0755); err != nil {
		return stacktrace.Propagate(err, "Failed to create output directory: %s", e.outputDir)
	}

	// Export table-specific objects
	if err := e.exportTableObjects(tableObjects); err != nil {
		return err
	}

	// Export standalone objects
	if err := e.exportStandaloneObjects(standalone); err != nil {
		return err
	}

	log.Info("Successfully exported all objects")
	return nil
}

// exportTableObjects exports table-related objects
func (e *Exporter) exportTableObjects(tableObjects map[string][]types.DBObject) error {
	for tableName, objs := range tableObjects {
		// Create table directory
		tableDir := filepath.Join(e.outputDir, "tables", tableName)
		if err := os.MkdirAll(tableDir, 0755); err != nil {
			return stacktrace.Propagate(err, "Failed to create table directory: %s", tableDir)
		}

		for _, obj := range objs {
			switch obj.Type {
			case types.TypeTable:
				// Save table definition
				tablePath := filepath.Join(tableDir, "table.sql")
				log.Debug("Writing table definition to %s", tablePath)
				if err := os.WriteFile(tablePath, []byte(obj.Definition), 0644); err != nil {
					return stacktrace.Propagate(err, "Failed to write table definition for %s", tableName)
				}

			case types.TypeTrigger:
				triggerDir := filepath.Join(tableDir, "triggers")
				if err := os.MkdirAll(triggerDir, 0755); err != nil {
					return stacktrace.Propagate(err, "Failed to create triggers directory for table %s", tableName)
				}
				filename := filepath.Join(triggerDir, fmt.Sprintf("%s.sql", obj.Name))
				log.Debug("Writing trigger definition to %s", filename)
				if err := os.WriteFile(filename, []byte(obj.Definition), 0644); err != nil {
					return stacktrace.Propagate(err, "Failed to write trigger definition for %s.%s", tableName, obj.Name)
				}

			case types.TypeIndex:
				indexDir := filepath.Join(tableDir, "indexes")
				if err := os.MkdirAll(indexDir, 0755); err != nil {
					return stacktrace.Propagate(err, "Failed to create indexes directory for table %s", tableName)
				}
				filename := filepath.Join(indexDir, fmt.Sprintf("%s.sql", obj.Name))
				log.Debug("Writing index definition to %s", filename)
				if err := os.WriteFile(filename, []byte(obj.Definition), 0644); err != nil {
					return stacktrace.Propagate(err, "Failed to write index definition for %s.%s", tableName, obj.Name)
				}

			case types.TypeConstraint:
				constraintDir := filepath.Join(tableDir, "constraints")
				if err := os.MkdirAll(constraintDir, 0755); err != nil {
					return stacktrace.Propagate(err, "Failed to create constraints directory for table %s", tableName)
				}
				filename := filepath.Join(constraintDir, fmt.Sprintf("%s.sql", obj.Name))
				log.Debug("Writing constraint definition to %s", filename)
				if err := os.WriteFile(filename, []byte(obj.Definition), 0644); err != nil {
					return stacktrace.Propagate(err, "Failed to write constraint definition for %s.%s", tableName, obj.Name)
				}
			}
		}
	}
	return nil
}

// exportStandaloneObjects exports standalone objects like functions and views
func (e *Exporter) exportStandaloneObjects(objects []types.DBObject) error {
	for _, obj := range objects {
		dir := filepath.Join(e.outputDir, string(obj.Type)+"s")
		if err := os.MkdirAll(dir, 0755); err != nil {
			return stacktrace.Propagate(err, "Failed to create directory: %s", dir)
		}
		filename := filepath.Join(dir, fmt.Sprintf("%s.%s.sql", obj.Schema, obj.Name))
		log.Debug("Writing %s definition to %s", obj.Type, filename)
		if err := os.WriteFile(filename, []byte(obj.Definition), 0644); err != nil {
			return stacktrace.Propagate(err, "Failed to write %s definition for %s.%s", obj.Type, obj.Schema, obj.Name)
		}
	}
	return nil
}