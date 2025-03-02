package export

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/palantir/stacktrace"
	"github.com/shkamensky/pgmeta/internal/log"
	"github.com/shkamensky/pgmeta/internal/metadata/db"
	"github.com/shkamensky/pgmeta/internal/metadata/types"
)

// Define the interface we need from the connector
type DBConnector interface {
	FetchObjectDefinition(ctx context.Context, obj *types.DBObject) error
	FetchObjectsDefinitionsConcurrently(ctx context.Context, objects []types.DBObject, concurrency int) ([]types.DBObject, []string, error)
}

// Exporter handles exporting database objects to files
type Exporter struct {
	connector      DBConnector
	outputDir      string
	concurrency    int
	dirMutexes     sync.Map // Used to synchronize directory creation
}

// New creates a new exporter with default concurrency
func New(connector *db.Connector, outputDir string) *Exporter {
	return &Exporter{
		connector:      connector,
		outputDir:      outputDir,
		concurrency:    50, // Default number of concurrent file operations
	}
}

// WithConcurrency sets the concurrency level for file operations
func (e *Exporter) WithConcurrency(n int) *Exporter {
	if n > 0 {
		e.concurrency = n
	}
	return e
}

// safelyMkdir creates a directory if it doesn't exist, using a mutex to prevent race conditions
func (e *Exporter) safelyMkdir(dir string) error {
	// Use a mutex for this specific directory to prevent race conditions
	// when multiple goroutines try to create the same directory
	key := dir
	mutex, _ := e.dirMutexes.LoadOrStore(key, &sync.Mutex{})
	mtx := mutex.(*sync.Mutex)
	
	mtx.Lock()
	defer mtx.Unlock()

	// Check if directory exists again under lock
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return stacktrace.Propagate(err, "Failed to create directory: %s", dir)
		}
	} else if err != nil {
		return stacktrace.Propagate(err, "Error checking directory: %s", dir)
	}
	return nil
}

// writeFile safely writes content to a file, creating parent directories if needed
func (e *Exporter) writeFile(path string, content []byte) error {
	// Create parent directory if it doesn't exist
	dir := filepath.Dir(path)
	if err := e.safelyMkdir(dir); err != nil {
		return err
	}
	
	// Write the file
	return os.WriteFile(path, content, 0644)
}

// ExportObjects exports database objects to files
func (e *Exporter) ExportObjects(ctx context.Context, objects []types.DBObject) error {
	startTime := time.Now()

	// Fetch all object definitions concurrently
	objectsWithDefs, failedObjects, err := e.connector.FetchObjectsDefinitionsConcurrently(ctx, objects, e.concurrency)
	if err != nil {
		return stacktrace.Propagate(err, "Failed to fetch object definitions")
	}

	// If any objects failed, don't proceed
	if len(failedObjects) > 0 {
		return stacktrace.NewError("Failed to fetch definitions for objects: %v", failedObjects)
	}

	// Group objects by their tables
	tableObjects := make(map[string][]types.DBObject)
	standalone := make([]types.DBObject, 0)

	for _, obj := range objectsWithDefs {
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

	// Ensure output directory exists
	if err := e.safelyMkdir(e.outputDir); err != nil {
		return err
	}

	// Start with table objects, which are usually more numerous
	tableErr := e.exportTableObjects(tableObjects)
	if tableErr != nil {
		return tableErr
	}

	// Then export standalone objects
	standaloneErr := e.exportStandaloneObjects(standalone)
	if standaloneErr != nil {
		return standaloneErr
	}

	duration := time.Since(startTime)
	log.Info("Successfully exported %d objects in %v", len(objects), duration)
	return nil
}

// fileExportTask represents a single file to be written
type fileExportTask struct {
	path      string
	content   []byte
	objType   types.ObjectType
	tableName string
	objName   string
}

// exportTableObjects exports table-related objects using concurrency
func (e *Exporter) exportTableObjects(tableObjects map[string][]types.DBObject) error {
	// Create a channel for file export tasks
	tasks := make(chan fileExportTask, len(tableObjects)*4) // Reasonable buffer size

	// Create a channel for errors
	errChan := make(chan error, 1)
	var wg sync.WaitGroup

	// Start worker goroutines
	for i := 0; i < e.concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for task := range tasks {
				// Create dir if not exists and write file
				log.Debug("Writing %s definition to %s", task.objType, task.path)
				if err := e.writeFile(task.path, task.content); err != nil {
					errMsg := ""
					switch {
					case task.objType == types.TypeTable:
						errMsg = fmt.Sprintf("Failed to write table definition for %s", task.tableName)
					case task.tableName != "":
						errMsg = fmt.Sprintf("Failed to write %s definition for %s.%s", 
							task.objType, task.tableName, task.objName)
					default:
						errMsg = fmt.Sprintf("Failed to write %s definition for %s", 
							task.objType, task.objName)
					}
					// Send the first error encountered
					select {
					case errChan <- stacktrace.Propagate(err, errMsg):
					default:
						// If channel already has an error, just log this one
						log.Error("%s: %v", errMsg, err)
					}
				}
			}
		}()
	}

	// Queue up all file write tasks
	for tableName, objs := range tableObjects {
		// Ensure table directory exists synchronously to avoid race conditions
		tableDir := filepath.Join(e.outputDir, "tables", tableName)
		if err := e.safelyMkdir(tableDir); err != nil {
			close(tasks) // Close channel to prevent goroutine leaks
			return stacktrace.Propagate(err, "Failed to create table directory: %s", tableDir)
		}

		for _, obj := range objs {
			switch obj.Type {
			case types.TypeTable:
				tablePath := filepath.Join(tableDir, "table.sql")
				tasks <- fileExportTask{
					path:      tablePath,
					content:   []byte(obj.Definition),
					objType:   types.TypeTable,
					tableName: tableName,
				}

			case types.TypeTrigger:
				triggerDir := filepath.Join(tableDir, "triggers")
				filename := filepath.Join(triggerDir, fmt.Sprintf("%s.sql", obj.Name))
				tasks <- fileExportTask{
					path:      filename,
					content:   []byte(obj.Definition),
					objType:   types.TypeTrigger,
					tableName: tableName,
					objName:   obj.Name,
				}

			case types.TypeIndex:
				indexDir := filepath.Join(tableDir, "indexes")
				filename := filepath.Join(indexDir, fmt.Sprintf("%s.sql", obj.Name))
				tasks <- fileExportTask{
					path:      filename,
					content:   []byte(obj.Definition),
					objType:   types.TypeIndex,
					tableName: tableName,
					objName:   obj.Name,
				}

			case types.TypeConstraint:
				constraintDir := filepath.Join(tableDir, "constraints")
				filename := filepath.Join(constraintDir, fmt.Sprintf("%s.sql", obj.Name))
				tasks <- fileExportTask{
					path:      filename,
					content:   []byte(obj.Definition),
					objType:   types.TypeConstraint,
					tableName: tableName,
					objName:   obj.Name,
				}
			}
		}
	}
	
	// Close the channel to signal no more tasks
	close(tasks)
	
	// Wait for all workers to finish
	wg.Wait()
	
	// Check if any errors were encountered
	select {
	case err := <-errChan:
		return err
	default:
		return nil
	}
}

// exportStandaloneObjects exports standalone objects like functions and views using concurrency
func (e *Exporter) exportStandaloneObjects(objects []types.DBObject) error {
	if len(objects) == 0 {
		return nil
	}

	// Create a channel for file export tasks
	tasks := make(chan fileExportTask, len(objects))
	
	// Create a channel for errors
	errChan := make(chan error, 1)
	var wg sync.WaitGroup

	// Start worker goroutines
	for i := 0; i < e.concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for task := range tasks {
				// Write file
				log.Debug("Writing %s definition to %s", task.objType, task.path)
				if err := e.writeFile(task.path, task.content); err != nil {
					// Send the first error encountered
					select {
					case errChan <- stacktrace.Propagate(err, "Failed to write %s definition for %s", 
						task.objType, task.objName):
					default:
						// If channel already has an error, just log this one
						log.Error("Failed to write %s definition for %s: %v", 
							task.objType, task.objName, err)
					}
				}
			}
		}()
	}

	// Group objects by type to ensure directories are created once
	typeGroups := make(map[types.ObjectType][]types.DBObject)
	for _, obj := range objects {
		typeGroups[obj.Type] = append(typeGroups[obj.Type], obj)
	}

	// Process each type group
	for objType, groupObjects := range typeGroups {
		// Create the directory for this object type
		dir := filepath.Join(e.outputDir, string(objType)+"s")
		if err := e.safelyMkdir(dir); err != nil {
			close(tasks) // Close channel to prevent goroutine leaks
			return stacktrace.Propagate(err, "Failed to create directory: %s", dir)
		}
		
		// Queue up all file write tasks for this type
		for _, obj := range groupObjects {
			filename := filepath.Join(dir, fmt.Sprintf("%s.%s.sql", obj.Schema, obj.Name))
			tasks <- fileExportTask{
				path:    filename,
				content: []byte(obj.Definition),
				objType: obj.Type,
				objName: fmt.Sprintf("%s.%s", obj.Schema, obj.Name),
			}
		}
	}
	
	// Close the channel to signal no more tasks
	close(tasks)
	
	// Wait for all workers to finish
	wg.Wait()
	
	// Check if any errors were encountered
	select {
	case err := <-errChan:
		return err
	default:
		return nil
	}
}