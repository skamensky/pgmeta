package db

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"
	"strings"
	"sync"

	"github.com/lib/pq"
	"github.com/palantir/stacktrace"
	"github.com/shkamensky/pgmeta/internal/log"
	"github.com/shkamensky/pgmeta/internal/metadata/types"
)

// Connector handles database connections
type Connector struct {
	db *sql.DB
}

// New creates a new database connector
func New(dbURL string) (*Connector, error) {
	// Use lib/pq's built-in URL parser
	connStr := dbURL
	if matched, _ := regexp.MatchString(`^postgres(ql)?://`, dbURL); matched {
		log.Debug("Converting URL to connection string: %s", dbURL)
		parsedURL, err := pq.ParseURL(dbURL)
		if err != nil {
			return nil, stacktrace.Propagate(err, "Failed to parse database URL: %s", dbURL)
		}
		connStr = parsedURL
	}

	// Open database connection
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, stacktrace.Propagate(err, "Failed to open database connection with connection string")
	}

	// Set reasonable defaults
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)

	// Try to ping the database
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, stacktrace.Propagate(err, "Failed to connect to database")
	}

	log.Info("Successfully connected to database")
	return &Connector{db: db}, nil
}

// Close closes the database connection
func (c *Connector) Close() error {
	if c.db != nil {
		log.Debug("Closing database connection")
		return c.db.Close()
	}
	return nil
}

// QueryObjects retrieves database objects matching the query options
func (c *Connector) QueryObjects(ctx context.Context, opts types.QueryOptions) ([]types.DBObject, error) {
	// Ensure we have at least one schema to work with
	if len(opts.Schemas) == 0 {
		opts.Schemas = []string{"public"}
	}

	pattern, err := regexp.Compile(opts.NameRegex)
	if err != nil {
		return nil, stacktrace.Propagate(err, "Invalid regex pattern: %s", opts.NameRegex)
	}

	var objects []types.DBObject

	// First let's verify all schemas exist
	for _, schema := range opts.Schemas {
		exists, err := c.schemaExists(ctx, schema)
		if err != nil {
			return nil, stacktrace.Propagate(err, "Failed to check if schema exists: %s", schema)
		}
		if !exists {
			return nil, stacktrace.NewError("Schema does not exist: %s", schema)
		}
	}

	// Loop through each schema and collect objects
	for _, schema := range opts.Schemas {
		log.Debug("Processing schema: %s", schema)

		// Query tables and views
		if types.ContainsAny(opts.Types, types.TypeTable, types.TypeView) {
			log.Debug("Querying tables and views in schema %s", schema)
			tables, err := c.queryTablesAndViews(ctx, schema, pattern)
			if err != nil {
				return nil, err
			}
			objects = append(objects, tables...)
		}

		// Query functions
		if types.ContainsAny(opts.Types, types.TypeFunction) {
			log.Debug("Querying functions in schema %s", schema)
			functions, err := c.queryFunctions(ctx, schema, pattern)
			if err != nil {
				return nil, err
			}
			objects = append(objects, functions...)
		}

		// Query triggers
		if types.ContainsAny(opts.Types, types.TypeTrigger) {
			log.Debug("Querying triggers in schema %s", schema)
			triggers, err := c.queryTriggers(ctx, schema, pattern)
			if err != nil {
				return nil, err
			}
			objects = append(objects, triggers...)
		}

		// Query indexes
		if types.ContainsAny(opts.Types, types.TypeIndex) {
			log.Debug("Querying indexes in schema %s", schema)
			indexes, err := c.queryIndexes(ctx, schema, pattern)
			if err != nil {
				return nil, err
			}
			objects = append(objects, indexes...)
		}

		// Query constraints
		if types.ContainsAny(opts.Types, types.TypeConstraint) {
			log.Debug("Querying constraints in schema %s", schema)
			constraints, err := c.queryConstraints(ctx, schema, pattern)
			if err != nil {
				return nil, err
			}
			objects = append(objects, constraints...)
		}
	}

	log.Info("Found %d database objects matching criteria", len(objects))
	return objects, nil
}

// queryTablesAndViews queries tables and views from the database
func (c *Connector) queryTablesAndViews(ctx context.Context, schema string, pattern *regexp.Regexp) ([]types.DBObject, error) {
	query := `
		SELECT 
			CASE WHEN table_type = 'BASE TABLE' THEN 'table' ELSE 'view' END as type,
			table_schema,
			table_name
		FROM information_schema.tables 
		WHERE table_schema = $1
	`
	rows, err := c.db.QueryContext(ctx, query, schema)
	if err != nil {
		return nil, stacktrace.Propagate(err, "Failed to query tables and views in schema: %s", schema)
	}
	defer rows.Close()

	var objects []types.DBObject
	for rows.Next() {
		var obj types.DBObject
		var typeStr string
		if err := rows.Scan(&typeStr, &obj.Schema, &obj.Name); err != nil {
			return nil, stacktrace.Propagate(err, "Failed to scan table/view row")
		}
		obj.Type = types.ObjectType(typeStr)
		if pattern.MatchString(obj.Name) {
			objects = append(objects, obj)
		}
	}
	return objects, nil
}

// queryFunctions queries functions from the database
func (c *Connector) queryFunctions(ctx context.Context, schema string, pattern *regexp.Regexp) ([]types.DBObject, error) {
	query := `
		SELECT 
			'function' as type,
			n.nspname as schema,
			p.proname as name
		FROM pg_proc p
		JOIN pg_namespace n ON n.oid = p.pronamespace
		WHERE n.nspname = $1
	`
	rows, err := c.db.QueryContext(ctx, query, schema)
	if err != nil {
		return nil, stacktrace.Propagate(err, "Failed to query functions in schema: %s", schema)
	}
	defer rows.Close()

	var objects []types.DBObject
	for rows.Next() {
		var obj types.DBObject
		var typeStr string
		if err := rows.Scan(&typeStr, &obj.Schema, &obj.Name); err != nil {
			return nil, stacktrace.Propagate(err, "Failed to scan function row")
		}
		obj.Type = types.ObjectType(typeStr)
		if pattern.MatchString(obj.Name) {
			objects = append(objects, obj)
		}
	}
	return objects, nil
}

// queryTriggers queries triggers from the database
func (c *Connector) queryTriggers(ctx context.Context, schema string, pattern *regexp.Regexp) ([]types.DBObject, error) {
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
	rows, err := c.db.QueryContext(ctx, query, schema)
	if err != nil {
		return nil, stacktrace.Propagate(err, "Failed to query triggers in schema: %s", schema)
	}
	defer rows.Close()

	var objects []types.DBObject
	for rows.Next() {
		var obj types.DBObject
		var typeStr string
		if err := rows.Scan(&typeStr, &obj.Schema, &obj.Name, &obj.TableName); err != nil {
			return nil, stacktrace.Propagate(err, "Failed to scan trigger row")
		}
		obj.Type = types.ObjectType(typeStr)
		if pattern.MatchString(obj.Name) {
			objects = append(objects, obj)
		}
	}
	return objects, nil
}

// queryIndexes queries indexes from the database
func (c *Connector) queryIndexes(ctx context.Context, schema string, pattern *regexp.Regexp) ([]types.DBObject, error) {
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
	rows, err := c.db.QueryContext(ctx, query, schema)
	if err != nil {
		return nil, stacktrace.Propagate(err, "Failed to query indexes in schema: %s", schema)
	}
	defer rows.Close()

	var objects []types.DBObject
	for rows.Next() {
		var obj types.DBObject
		var typeStr string
		if err := rows.Scan(&typeStr, &obj.Schema, &obj.Name, &obj.TableName); err != nil {
			return nil, stacktrace.Propagate(err, "Failed to scan index row")
		}
		obj.Type = types.ObjectType(typeStr)
		if pattern.MatchString(obj.Name) {
			objects = append(objects, obj)
		}
	}
	return objects, nil
}

// queryConstraints queries constraints from the database
func (c *Connector) queryConstraints(ctx context.Context, schema string, pattern *regexp.Regexp) ([]types.DBObject, error) {
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
	rows, err := c.db.QueryContext(ctx, query, schema)
	if err != nil {
		return nil, stacktrace.Propagate(err, "Failed to query constraints in schema: %s", schema)
	}
	defer rows.Close()

	var objects []types.DBObject
	for rows.Next() {
		var obj types.DBObject
		var typeStr string
		if err := rows.Scan(&typeStr, &obj.Schema, &obj.Name, &obj.TableName, &obj.Definition); err != nil {
			return nil, stacktrace.Propagate(err, "Failed to scan constraint row")
		}
		obj.Type = types.ObjectType(typeStr)
		if pattern.MatchString(obj.Name) {
			objects = append(objects, obj)
		}
	}
	return objects, nil
}

// FetchObjectDefinition fetches the SQL definition for a database object
func (c *Connector) FetchObjectDefinition(ctx context.Context, obj *types.DBObject) error {
	// If we already have the definition (like for constraints), return early
	if obj.Definition != "" {
		return nil
	}

	log.Debug("Fetching definition for %s %s.%s", obj.Type, obj.Schema, obj.Name)
	var query string
	var args []interface{}

	switch obj.Type {
	case types.TypeTable:
		query = buildTableDefinitionQuery()
		args = []interface{}{obj.Schema, obj.Name}
	case types.TypeView:
		query = `
			SELECT 
				'CREATE OR REPLACE VIEW ' || $1 || '.' || $2 || ' AS' || E'\n' ||
				view_definition
			FROM information_schema.views
			WHERE table_schema = $1 AND table_name = $2;
		`
		args = []interface{}{obj.Schema, obj.Name}
	case types.TypeFunction:
		query = `
			SELECT pg_get_functiondef(p.oid)
			FROM pg_proc p
			JOIN pg_namespace n ON n.oid = p.pronamespace
			WHERE n.nspname = $1 AND p.proname = $2;
		`
		args = []interface{}{obj.Schema, obj.Name}
	case types.TypeTrigger:
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
		args = []interface{}{obj.Schema, obj.Name}
	case types.TypeIndex:
		query = `
			SELECT pg_get_indexdef(i.indexrelid)
			FROM pg_index i
			JOIN pg_class c ON c.oid = i.indexrelid
			JOIN pg_namespace n ON n.oid = c.relnamespace
			WHERE n.nspname = $1 AND c.relname = $2;
		`
		args = []interface{}{obj.Schema, obj.Name}
	default:
		return stacktrace.NewError("Unsupported object type: %s", obj.Type)
	}

	var definition sql.NullString
	err := c.db.QueryRowContext(ctx, query, args...).Scan(&definition)
	if err != nil {
		if err == sql.ErrNoRows {
			return stacktrace.NewError("No definition found for %s.%s of type %s", obj.Schema, obj.Name, obj.Type)
		}
		return stacktrace.Propagate(err, "Database error when fetching definition for %s.%s", obj.Schema, obj.Name)
	}

	if !definition.Valid {
		return stacktrace.NewError("Definition is NULL for %s.%s of type %s", obj.Schema, obj.Name, obj.Type)
	}

	obj.Definition = definition.String
	return nil
}

// FetchObjectsDefinitionsConcurrently fetches definitions for multiple objects concurrently
func (c *Connector) FetchObjectsDefinitionsConcurrently(ctx context.Context, objects []types.DBObject, concurrency int) ([]types.DBObject, []string, error) {
	if concurrency <= 0 {
		concurrency = 10 // Default concurrency if invalid value provided
	}

	log.Info("Fetching definitions concurrently for %d objects with concurrency %d", len(objects), concurrency)
	
	results := make([]types.DBObject, len(objects))
	copy(results, objects) // Make a copy of the objects to avoid modifying the original slice
	
	var failedMutex sync.Mutex
	failedObjects := make([]string, 0)
	
	// Create a semaphore using a channel to limit concurrency
	sem := make(chan struct{}, concurrency)
	
	// Create a wait group to wait for all goroutines to finish
	var wg sync.WaitGroup
	
	// Process each object in a goroutine
	for i := range results {
		// Skip objects that already have definitions
		if results[i].Definition != "" {
			continue
		}
		
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			
			// Acquire a semaphore slot
			sem <- struct{}{}
			defer func() {
				// Release the semaphore slot
				<-sem
			}()
			
			// Fetch the definition for this object
			err := c.FetchObjectDefinition(ctx, &results[idx])
			if err != nil {
				failedMutex.Lock()
				failedObjects = append(failedObjects, fmt.Sprintf("%s.%s", results[idx].Schema, results[idx].Name))
				failedMutex.Unlock()
				log.Warn("Failed to fetch definition for %s.%s: %v", results[idx].Schema, results[idx].Name, err)
			}
		}(i)
	}
	
	// Wait for all goroutines to finish
	wg.Wait()
	
	return results, failedObjects, nil
}

// buildTableDefinitionQuery creates the SQL query for table definition
func buildTableDefinitionQuery() string {
	return strings.TrimSpace(`
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
	`)
}

// schemaExists checks if the given schema exists in the database
func (c *Connector) schemaExists(ctx context.Context, schema string) (bool, error) {
	query := `
		SELECT EXISTS (
			SELECT 1 FROM information_schema.schemata 
			WHERE schema_name = $1
		);
	`
	var exists bool
	err := c.db.QueryRowContext(ctx, query, schema).Scan(&exists)
	if err != nil {
		return false, stacktrace.Propagate(err, "Failed to check if schema exists: %s", schema)
	}
	return exists, nil
}

// GetAllSchemas returns a list of all schemas in the database
func (c *Connector) GetAllSchemas(ctx context.Context) ([]string, error) {
	query := `
		SELECT schema_name
		FROM information_schema.schemata
		WHERE schema_name NOT LIKE 'pg_%'
		AND schema_name != 'information_schema'
		ORDER BY schema_name;
	`
	rows, err := c.db.QueryContext(ctx, query)
	if err != nil {
		return nil, stacktrace.Propagate(err, "Failed to query schemas")
	}
	defer rows.Close()

	var schemas []string
	for rows.Next() {
		var schema string
		if err := rows.Scan(&schema); err != nil {
			return nil, stacktrace.Propagate(err, "Failed to scan schema row")
		}
		schemas = append(schemas, schema)
	}
	return schemas, nil
}