package db

import (
	"context"
	"database/sql"
	"regexp"
	"strings"

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
	if opts.Schema == "" {
		opts.Schema = "public"
	}

	pattern, err := regexp.Compile(opts.NameRegex)
	if err != nil {
		return nil, stacktrace.Propagate(err, "Invalid regex pattern: %s", opts.NameRegex)
	}

	var objects []types.DBObject

	// Query tables and views
	if types.ContainsAny(opts.Types, types.TypeTable, types.TypeView) {
		log.Debug("Querying tables and views in schema %s", opts.Schema)
		tables, err := c.queryTablesAndViews(ctx, opts.Schema, pattern)
		if err != nil {
			return nil, err
		}
		objects = append(objects, tables...)
	}

	// Query functions
	if types.ContainsAny(opts.Types, types.TypeFunction) {
		log.Debug("Querying functions in schema %s", opts.Schema)
		functions, err := c.queryFunctions(ctx, opts.Schema, pattern)
		if err != nil {
			return nil, err
		}
		objects = append(objects, functions...)
	}

	// Query triggers
	if types.ContainsAny(opts.Types, types.TypeTrigger) {
		log.Debug("Querying triggers in schema %s", opts.Schema)
		triggers, err := c.queryTriggers(ctx, opts.Schema, pattern)
		if err != nil {
			return nil, err
		}
		objects = append(objects, triggers...)
	}

	// Query indexes
	if types.ContainsAny(opts.Types, types.TypeIndex) {
		log.Debug("Querying indexes in schema %s", opts.Schema)
		indexes, err := c.queryIndexes(ctx, opts.Schema, pattern)
		if err != nil {
			return nil, err
		}
		objects = append(objects, indexes...)
	}

	// Query constraints
	if types.ContainsAny(opts.Types, types.TypeConstraint) {
		log.Debug("Querying constraints in schema %s", opts.Schema)
		constraints, err := c.queryConstraints(ctx, opts.Schema, pattern)
		if err != nil {
			return nil, err
		}
		objects = append(objects, constraints...)
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