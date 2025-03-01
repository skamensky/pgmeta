package metadata

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	_ "github.com/lib/pq"
	"github.com/palantir/stacktrace"
	"github.com/lib/pq"
)

// ObjectType represents the type of database object
type ObjectType string

const (
	TypeTable     ObjectType = "table"
	TypeView      ObjectType = "view"
	TypeFunction  ObjectType = "function"
	TypeTrigger   ObjectType = "trigger"
	TypeIndex     ObjectType = "index"
	TypeConstraint ObjectType = "constraint" // Add constraint type
)

// DBObject represents a database object
type DBObject struct {
	Type       ObjectType
	Schema     string
	Name       string
	Definition string
	TableName  string // Add this field to store the parent table name for indexes and triggers
}

type QueryOptions struct {
	Types      []ObjectType
	Schema     string
	Database   string
	NameRegex  string
}

// Fetcher handles PostgreSQL metadata retrieval
type Fetcher struct {
	db *sql.DB
}
// NewFetcher creates a new metadata fetcher instance
func NewFetcher(dbURL string) (*Fetcher, error) {
	// Use lib/pq's built-in URL parser
	connStr := dbURL
	if matched, _ := regexp.MatchString(`^postgresql://`, dbURL); matched {
		parsedURL, err := pq.ParseURL(dbURL)
		if err != nil {
			return nil, stacktrace.Propagate(err, "Failed to parse database URL")
		}
		connStr = parsedURL
	}

	// Open database connection
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, stacktrace.Propagate(err, "Failed to open database connection")
	}

	// Try to ping the database
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, stacktrace.Propagate(err, "Failed to ping database")
	}

	return &Fetcher{db: db}, nil
}

// Close closes the database connection
func (f *Fetcher) Close() error {
	return f.db.Close()
}

func (f *Fetcher) QueryObjects(opts QueryOptions) ([]DBObject, error) {
	if opts.Schema == "" {
		opts.Schema = "public"
	}

	pattern, err := regexp.Compile(opts.NameRegex)
	if err != nil {
		return nil, fmt.Errorf("invalid regex pattern: %w", err)
	}

	var objects []DBObject

	// Query tables and views
	if containsAny(opts.Types, TypeTable, TypeView) {
		query := `
			SELECT 
				CASE WHEN table_type = 'BASE TABLE' THEN 'table' ELSE 'view' END as type,
				table_schema,
				table_name
			FROM information_schema.tables 
			WHERE table_schema = $1
		`
		rows, err := f.db.Query(query, opts.Schema)
		if err != nil {
			return nil, err
		}
		defer rows.Close()

		for rows.Next() {
			var obj DBObject
			if err := rows.Scan(&obj.Type, &obj.Schema, &obj.Name); err != nil {
				return nil, err
			}
			if pattern.MatchString(obj.Name) {
				objects = append(objects, obj)
			}
		}
	}

	// Query functions
	if containsAny(opts.Types, TypeFunction) {
		query := `
			SELECT 
				'function' as type,
				n.nspname as schema,
				p.proname as name
			FROM pg_proc p
			JOIN pg_namespace n ON n.oid = p.pronamespace
			WHERE n.nspname = $1
		`
		rows, err := f.db.Query(query, opts.Schema)
		if err != nil {
			return nil, stacktrace.Propagate(err, "Failed to query functions")
		}
		defer rows.Close()

		for rows.Next() {
			var obj DBObject
			if err := rows.Scan(&obj.Type, &obj.Schema, &obj.Name); err != nil {
				return nil, stacktrace.Propagate(err, "Failed to scan function row")
			}
			if pattern.MatchString(obj.Name) {
				objects = append(objects, obj)
			}
		}
	}

	// Query triggers
	if containsAny(opts.Types, TypeTrigger) {
		query := `
			SELECT 
				'trigger' as type,
				n.nspname as schema,
				t.tgname as name,
				c.relname as table_name
			FROM pg_trigger t
			JOIN pg_class c ON t.tgrelid = c.oid
			JOIN pg_namespace n ON c.relnamespace = n.oid
			WHERE n.nspname = $1
			AND NOT t.tgisinternal
		`
		rows, err := f.db.Query(query, opts.Schema)
		if err != nil {
			return nil, stacktrace.Propagate(err, "Failed to query triggers")
		}
		defer rows.Close()

		for rows.Next() {
			var obj DBObject
			if err := rows.Scan(&obj.Type, &obj.Schema, &obj.Name, &obj.TableName); err != nil {
				return nil, stacktrace.Propagate(err, "Failed to scan trigger row")
			}
			if pattern.MatchString(obj.Name) {
				objects = append(objects, obj)
			}
		}
	}

	// Query indexes
	if containsAny(opts.Types, TypeIndex) {
		query := `
			SELECT 
				'index' as type,
				n.nspname as schema,
				c.relname as name,
				t.relname as table_name
			FROM pg_index i
			JOIN pg_class c ON c.oid = i.indexrelid
			JOIN pg_class t ON t.oid = i.indrelid
			JOIN pg_namespace n ON n.oid = c.relnamespace
			WHERE n.nspname = $1
			AND t.relkind = 'r'
		`
		rows, err := f.db.Query(query, opts.Schema)
		if err != nil {
			return nil, stacktrace.Propagate(err, "Failed to query indexes")
		}
		defer rows.Close()

		for rows.Next() {
			var obj DBObject
			if err := rows.Scan(&obj.Type, &obj.Schema, &obj.Name, &obj.TableName); err != nil {
				return nil, stacktrace.Propagate(err, "Failed to scan index row")
			}
			if pattern.MatchString(obj.Name) {
				objects = append(objects, obj)
			}
		}
	}

	// Query constraints (primary keys, foreign keys, unique, check)
	if containsAny(opts.Types, TypeConstraint) {
		query := `
			SELECT 
				'constraint' as type,
				n.nspname as schema,
				c.conname as name,
				rel.relname as table_name,
				pg_get_constraintdef(c.oid) as definition
			FROM pg_constraint c
			JOIN pg_class rel ON rel.oid = c.conrelid
			JOIN pg_namespace n ON n.oid = rel.relnamespace
			WHERE n.nspname = $1
			AND c.contype IN ('p', 'f', 'u', 'c')  -- primary, foreign, unique, check
		`
		rows, err := f.db.Query(query, opts.Schema)
		if err != nil {
			return nil, stacktrace.Propagate(err, "Failed to query constraints")
		}
		defer rows.Close()

		for rows.Next() {
			var obj DBObject
			var tempDef string
			if err := rows.Scan(&obj.Type, &obj.Schema, &obj.Name, &obj.TableName, &tempDef); err != nil {
				return nil, stacktrace.Propagate(err, "Failed to scan constraint row")
			}
			if pattern.MatchString(obj.Name) {
				// Store the definition directly since we already have it
				obj.Definition = tempDef
				objects = append(objects, obj)
			}
		}
	}

	return objects, nil
}

func (f *Fetcher) FetchObjectDefinition(obj *DBObject) error {
	// If we already have the definition (like for constraints), return early
	if obj.Definition != "" {
		return nil
	}

	var query string
	switch obj.Type {
	case TypeTable:
		query = `
			WITH columns AS (
				SELECT 
					column_name,
					data_type,
					CASE 
						WHEN character_maximum_length IS NOT NULL THEN '(' || character_maximum_length || ')'
						WHEN numeric_precision IS NOT NULL THEN '(' || numeric_precision || 
							CASE WHEN numeric_scale IS NOT NULL THEN ',' || numeric_scale ELSE '' END || ')'
						ELSE ''
					END as size,
					is_nullable,
					column_default
				FROM information_schema.columns 
				WHERE table_schema = $1 AND table_name = $2
				ORDER BY ordinal_position
			),
			constraints AS (
				SELECT 
					pg_get_constraintdef(c.oid) as definition
				FROM pg_constraint c
				JOIN pg_namespace n ON n.oid = c.connamespace
				WHERE n.nspname = $1 AND c.conrelid::regclass::text = $1 || '.' || $2
			)
			SELECT 
				'CREATE TABLE ' || $1 || '.' || $2 || ' (' || E'\n' ||
				(SELECT string_agg(
					'    ' || column_name || ' ' || data_type || size || 
					CASE WHEN is_nullable = 'NO' THEN ' NOT NULL' ELSE '' END ||
					CASE WHEN column_default IS NOT NULL THEN ' DEFAULT ' || column_default ELSE '' END,
					E',\n'
				) FROM columns) ||
				COALESCE((
					SELECT E',\n    ' || string_agg(definition, E',\n    ')
					FROM constraints
					WHERE EXISTS (SELECT 1 FROM constraints)
				), '') ||
				E'\n);'
		`
	case TypeView:
		query = `
			SELECT 
				'CREATE OR REPLACE VIEW ' || $1 || '.' || $2 || ' AS' || E'\n' ||
				view_definition
			FROM information_schema.views
			WHERE table_schema = $1 AND table_name = $2;
		`
	case TypeFunction:
		query = `
			SELECT pg_get_functiondef(p.oid)
			FROM pg_proc p
			JOIN pg_namespace n ON n.oid = p.pronamespace
			WHERE n.nspname = $1 AND p.proname = $2;
		`
	case TypeTrigger:
		query = `
			SELECT 
				'CREATE TRIGGER ' || trigger_name || E'\n' ||
				action_timing || ' ' || event_manipulation || E'\n' ||
				'ON ' || event_object_schema || '.' || event_object_table || E'\n' ||
				'FOR EACH ' || action_orientation || E'\n' ||
				action_statement || ';'
			FROM information_schema.triggers
			WHERE trigger_schema = $1 AND trigger_name = $2;
		`
	case TypeIndex:
		query = `
			SELECT pg_get_indexdef(i.indexrelid)
			FROM pg_index i
			JOIN pg_class c ON c.oid = i.indexrelid
			JOIN pg_namespace n ON n.oid = c.relnamespace
			WHERE n.nspname = $1 AND c.relname = $2;
		`
	default:
		return fmt.Errorf("unsupported object type: %s", obj.Type)
	}

	var definition sql.NullString
	err := f.db.QueryRow(query, obj.Schema, obj.Name).Scan(&definition)
	if err != nil {
		return stacktrace.Propagate(err, "Failed to fetch definition")
	}

	if !definition.Valid {
		return fmt.Errorf("no definition found for %s.%s", obj.Schema, obj.Name)
	}

	obj.Definition = definition.String
	return nil
}

func (f *Fetcher) SaveObjects(objects []DBObject, outputDir string) error {
	// Track if any objects failed
	hasFailures := false
	failedObjects := make([]string, 0)

	// Group objects by their tables
	tableObjects := make(map[string][]DBObject)
	standalone := make([]DBObject, 0)

	for _, obj := range objects {
		// Fetch definition for all objects
		if err := f.FetchObjectDefinition(&obj); err != nil {
			hasFailures = true
			failedObjects = append(failedObjects, fmt.Sprintf("%s.%s", obj.Schema, obj.Name))
			fmt.Printf("Warning: failed to fetch definition for %s.%s: %v\n", 
				obj.Schema, obj.Name, err)
			continue
		}

		switch obj.Type {
		case TypeTable:
			tableObjects[obj.Name] = append(tableObjects[obj.Name], obj)
		case TypeTrigger, TypeIndex, TypeConstraint: // Add constraint here
			// Use the TableName field we populated during query
			if obj.TableName != "" {
				tableObjects[obj.TableName] = append(tableObjects[obj.TableName], obj)
			} else {
				fmt.Printf("Warning: %s %s has no associated table name\n", obj.Type, obj.Name)
				standalone = append(standalone, obj)
			}
		default:
			standalone = append(standalone, obj)
		}
	}

	// Don't save anything if there were failures
	if hasFailures {
		return fmt.Errorf("failed to fetch definitions for objects: %v", failedObjects)
	}

	// Save table-specific objects
	for tableName, objs := range tableObjects {
		// Create table directory
		tableDir := filepath.Join(outputDir, "tables", tableName)
		if err := os.MkdirAll(tableDir, 0755); err != nil {
			return stacktrace.Propagate(err, "Failed to create table directory")
		}

		for _, obj := range objs {
			switch obj.Type {
			case TypeTable:
				// Save table definition
				if err := os.WriteFile(filepath.Join(tableDir, "table.sql"), []byte(obj.Definition), 0644); err != nil {
					return stacktrace.Propagate(err, "Failed to write table definition")
				}
			case TypeTrigger:
				triggerDir := filepath.Join(tableDir, "triggers")
				if err := os.MkdirAll(triggerDir, 0755); err != nil {
					return stacktrace.Propagate(err, "Failed to create triggers directory")
				}
				filename := filepath.Join(triggerDir, fmt.Sprintf("%s.sql", obj.Name))
				if err := os.WriteFile(filename, []byte(obj.Definition), 0644); err != nil {
					return stacktrace.Propagate(err, "Failed to write trigger definition")
				}
			case TypeIndex:
				indexDir := filepath.Join(tableDir, "indexes")
				if err := os.MkdirAll(indexDir, 0755); err != nil {
					return stacktrace.Propagate(err, "Failed to create indexes directory")
				}
				filename := filepath.Join(indexDir, fmt.Sprintf("%s.sql", obj.Name))
				if err := os.WriteFile(filename, []byte(obj.Definition), 0644); err != nil {
					return stacktrace.Propagate(err, "Failed to write index definition")
				}
			case TypeConstraint:
				constraintDir := filepath.Join(tableDir, "constraints")
				if err := os.MkdirAll(constraintDir, 0755); err != nil {
					return stacktrace.Propagate(err, "Failed to create constraints directory")
				}
				filename := filepath.Join(constraintDir, fmt.Sprintf("%s.sql", obj.Name))
				if err := os.WriteFile(filename, []byte(obj.Definition), 0644); err != nil {
					return stacktrace.Propagate(err, "Failed to write constraint definition")
				}
			}
		}
	}

	// Save standalone objects (functions, views)
	for _, obj := range standalone {
		dir := filepath.Join(outputDir, string(obj.Type)+"s")
		if err := os.MkdirAll(dir, 0755); err != nil {
			return stacktrace.Propagate(err, "Failed to create directory")
		}
		filename := filepath.Join(dir, fmt.Sprintf("%s.%s.sql", obj.Schema, obj.Name))
		if err := os.WriteFile(filename, []byte(obj.Definition), 0644); err != nil {
			return stacktrace.Propagate(err, "Failed to write object definition")
		}
	}

	return nil
}

func containsAny(slice []ObjectType, elements ...ObjectType) bool {
	if len(slice) == 0 {
		return true // If no types specified, include all
	}
	for _, s := range slice {
		for _, e := range elements {
			if s == e {
				return true
			}
		}
	}
	return false
}

func isValidType(t ObjectType) bool {
	validTypes := map[ObjectType]bool{
		TypeTable:      true,
		TypeView:       true,
		TypeFunction:   true,
		TypeTrigger:    true,
		TypeIndex:      true,
		TypeConstraint: true,
	}
	return validTypes[t]
}