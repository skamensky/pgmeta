package db

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"unicode"

	"github.com/lib/pq"
	"github.com/palantir/stacktrace"
	"github.com/skamensky/pgmeta/internal/log"
	"github.com/skamensky/pgmeta/internal/metadata/types"
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

		// Query sequences
		if types.ContainsAny(opts.Types, types.TypeSequence) {
			log.Debug("Querying sequences in schema %s", schema)
			sequences, err := c.querySequences(ctx, schema, pattern)
			if err != nil {
				return nil, err
			}
			objects = append(objects, sequences...)
		}

		// Query materialized views
		if types.ContainsAny(opts.Types, types.TypeMaterializedView) {
			log.Debug("Querying materialized views in schema %s", schema)
			matViews, err := c.queryMaterializedViews(ctx, schema, pattern)
			if err != nil {
				return nil, err
			}
			objects = append(objects, matViews...)
		}

		// Query policies
		if types.ContainsAny(opts.Types, types.TypePolicy) {
			log.Debug("Querying policies in schema %s", schema)
			policies, err := c.queryPolicies(ctx, schema, pattern)
			if err != nil {
				return nil, err
			}
			objects = append(objects, policies...)
		}

		// Query extensions
		if types.ContainsAny(opts.Types, types.TypeExtension) {
			log.Debug("Querying extensions in schema %s", schema)
			extensions, err := c.queryExtensions(ctx, schema, pattern)
			if err != nil {
				return nil, err
			}
			objects = append(objects, extensions...)
		}

		// Query procedures
		if types.ContainsAny(opts.Types, types.TypeProcedure) {
			log.Debug("Querying procedures in schema %s", schema)
			procedures, err := c.queryProcedures(ctx, schema, pattern)
			if err != nil {
				return nil, err
			}
			objects = append(objects, procedures...)
		}

		// Query rules
		if types.ContainsAny(opts.Types, types.TypeRule) {
			log.Debug("Querying rules in schema %s", schema)
			rules, err := c.queryRules(ctx, schema, pattern)
			if err != nil {
				return nil, err
			}
			objects = append(objects, rules...)
		}

		// Query aggregates
		if types.ContainsAny(opts.Types, types.TypeAggregate) {
			log.Debug("Querying aggregates in schema %s", schema)
			aggregates, err := c.queryAggregates(ctx, schema, pattern)
			if err != nil {
				return nil, err
			}
			objects = append(objects, aggregates...)
		}
	}

	// Query database-level objects (outside of schema loop)
	// These only need to be queried once, not per schema

	// Query publications
	if types.ContainsAny(opts.Types, types.TypePublication) {
		log.Debug("Querying publications")
		publications, err := c.queryPublications(ctx, pattern)
		if err != nil {
			return nil, err
		}
		objects = append(objects, publications...)
	}

	// Query subscriptions
	if types.ContainsAny(opts.Types, types.TypeSubscription) {
		log.Debug("Querying subscriptions")
		subscriptions, err := c.querySubscriptions(ctx, pattern)
		if err != nil {
			return nil, err
		}
		objects = append(objects, subscriptions...)
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
		WHERE table_schema = ($1)::text
		AND table_type IN ('BASE TABLE', 'VIEW')
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
		WHERE n.nspname = ($1)::text
		AND p.prokind = 'f'  -- Only normal functions
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

// queryAggregates queries aggregates from the database
func (c *Connector) queryAggregates(ctx context.Context, schema string, pattern *regexp.Regexp) ([]types.DBObject, error) {
	query := `
		SELECT 
			'aggregate' as type,
			n.nspname as schema,
			p.proname as name
		FROM pg_proc p
		JOIN pg_namespace n ON n.oid = p.pronamespace
		WHERE n.nspname = ($1)::text AND
		p.prokind = 'a'
	`
	rows, err := c.db.QueryContext(ctx, query, schema)
	if err != nil {
		return nil, stacktrace.Propagate(err, "Failed to query aggregates in schema: %s", schema)
	}
	defer rows.Close()

	var objects []types.DBObject
	for rows.Next() {
		var obj types.DBObject
		var typeStr string
		if err := rows.Scan(&typeStr, &obj.Schema, &obj.Name); err != nil {
			return nil, stacktrace.Propagate(err, "Failed to scan aggregate row")
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
		WHERE n.nspname = ($1)::text
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
		WHERE n.nspname = ($1)::text
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
		WHERE n.nspname = ($1)::text
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
			SELECT COALESCE(
				-- Try information_schema.views first
				(SELECT 'CREATE OR REPLACE VIEW ' || quote_ident($1) || '.' || quote_ident($2) || ' AS' || E'\n' ||
					view_definition
				FROM information_schema.views
				WHERE table_schema = $1 AND table_name = $2),
				-- Fall back to pg_get_viewdef for system/extension views
				(SELECT 'CREATE OR REPLACE VIEW ' || quote_ident($1) || '.' || quote_ident($2) || ' AS' || E'\n' ||
					pg_get_viewdef(quote_ident($1) || '.' || quote_ident($2), true)
				FROM pg_class c
				JOIN pg_namespace n ON n.oid = c.relnamespace
				WHERE n.nspname = $1 AND c.relname = $2 AND c.relkind = 'v')
			);
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
			SELECT pg_get_triggerdef(t.oid)
			FROM pg_trigger t
			JOIN pg_class c ON t.tgrelid = c.oid
			JOIN pg_namespace n ON c.relnamespace = n.oid
			WHERE n.nspname = $1 
			AND t.tgname = $2
			AND NOT t.tgisinternal;
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
	case types.TypeSequence:
		query = `
			SELECT 
				'CREATE SEQUENCE ' || quote_ident($1) || '.' || quote_ident($2) || E'\n' ||
				CASE WHEN s.increment::bigint <> 1 THEN '    INCREMENT BY ' || s.increment || E'\n' ELSE '' END ||
				'    START WITH ' || s.start_value || E'\n' ||
				'    MINVALUE ' || s.minimum_value || E'\n' ||
				'    MAXVALUE ' || s.maximum_value || E'\n' ||
				CASE WHEN NOT s.cycle_option='YES' THEN '    NO' ELSE '' END || ' CYCLE;'
			FROM information_schema.sequences s
			WHERE s.sequence_schema = $1 AND s.sequence_name = $2;
		`
		args = []interface{}{obj.Schema, obj.Name}
	case types.TypeMaterializedView:
		query = `
			SELECT 'CREATE MATERIALIZED VIEW ' || quote_ident($1) || '.' || quote_ident($2) || ' AS' || E'\n' || 
				pg_get_viewdef(c.oid, true)
			FROM pg_class c
			JOIN pg_namespace n ON n.oid = c.relnamespace
			WHERE c.relkind = 'm'
			AND n.nspname = $1 AND c.relname = $2;
		`
		args = []interface{}{obj.Schema, obj.Name}
	case types.TypePolicy:
		query = `
			WITH policy_info AS (
				SELECT 
					pol.polname AS name,
					c.relname AS table_name,
					n.nspname AS schema_name,
					CASE pol.polcmd
						WHEN 'r' THEN 'SELECT'
						WHEN 'a' THEN 'INSERT'
						WHEN 'w' THEN 'UPDATE'
						WHEN 'd' THEN 'DELETE'
						WHEN '*' THEN 'ALL'
					END AS command,
					pg_get_expr(pol.polqual, pol.polrelid) AS using_expr,
					pg_get_expr(pol.polwithcheck, pol.polrelid) AS check_expr,
					ARRAY(
						SELECT pg_get_userbyid(member)
						FROM unnest(pol.polroles) AS member
					) AS roles
				FROM pg_policy pol
				JOIN pg_class c ON pol.polrelid = c.oid
				JOIN pg_namespace n ON c.relnamespace = n.oid
				WHERE n.nspname = $1 AND pol.polname = $2
			)
			SELECT 
				'CREATE POLICY ' || quote_ident(name) || ' ON ' || 
				quote_ident(schema_name) || '.' || quote_ident(table_name) || 
				' FOR ' || command || 
				' TO ' || (
					CASE 
						WHEN array_position(roles, 'public') IS NOT NULL THEN 'PUBLIC'
						ELSE array_to_string(roles, ', ')
					END
				) ||
				CASE WHEN using_expr IS NOT NULL THEN E'\n  USING (' || using_expr || ')' ELSE '' END ||
				CASE WHEN check_expr IS NOT NULL THEN E'\n  WITH CHECK (' || check_expr || ')' ELSE '' END ||
				';'
			FROM policy_info;
		`
		args = []interface{}{obj.Schema, obj.Name}
	case types.TypeExtension:
		query = `
			SELECT 'CREATE EXTENSION IF NOT EXISTS ' || quote_ident(extname) || ';'
			FROM pg_extension
			WHERE extname = $1;
		`
		args = []interface{}{obj.Name}
	case types.TypeProcedure:
		query = `
			SELECT pg_get_functiondef(p.oid)
			FROM pg_proc p
			JOIN pg_namespace n ON n.oid = p.pronamespace
			WHERE p.prokind = 'p'
			AND n.nspname = $1 AND p.proname = $2;
		`
		args = []interface{}{obj.Schema, obj.Name}
	case types.TypePublication:
		query = `
			SELECT 
				'CREATE PUBLICATION ' || quote_ident(p.pubname) || 
				CASE
					WHEN p.puballtables THEN ' FOR ALL TABLES;'
					ELSE
						COALESCE(
							(SELECT ' FOR TABLE ' || 
								string_agg(quote_ident(t.schemaname) || '.' || quote_ident(t.tablename), ', ')
							FROM pg_publication_tables t
							WHERE t.pubname = p.pubname),
							''
						) || ';'
				END
			FROM pg_publication p
			WHERE p.pubname = $1;
		`
		args = []interface{}{obj.Name}
	case types.TypeSubscription:
		query = `
			WITH sub_details AS (
				SELECT 
					s.subname,
					s.subconninfo,
					(SELECT array_agg(pub) FROM unnest(s.subpublications) AS pub) AS pubs
				FROM pg_subscription s
				WHERE s.subname = $1
			)
			SELECT 
				'CREATE SUBSCRIPTION ' || quote_ident(subname) || 
				' CONNECTION ''' || subconninfo || '''' ||
				' PUBLICATION ' || array_to_string(pubs, ', ') || ';'
			FROM sub_details;
		`
		args = []interface{}{obj.Name}
	case types.TypeRule:
		query = `
			SELECT pg_get_ruledef(r.oid)
			FROM pg_rewrite r
			JOIN pg_class c ON r.ev_class = c.oid
			JOIN pg_namespace n ON c.relnamespace = n.oid
			WHERE r.rulename != '_RETURN'
			AND n.nspname = $1 AND r.rulename = $2;
		`
		args = []interface{}{obj.Schema, obj.Name}
	case types.TypeAggregate:
		query = `
			SELECT format(
				'CREATE AGGREGATE %I.%I (%s) (SFUNC = %I, STYPE = %s)',
				n.nspname,
				p.proname,
				pg_get_function_arguments(p.oid),
				p.proname || '_sfunc',
				format_type(p.proargtypes[0], NULL)
			)
			FROM pg_proc p
			JOIN pg_namespace n ON n.oid = p.pronamespace
			WHERE n.nspname = $1 
			AND p.proname = $2
			AND p.prokind = 'a';
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
				log.Warn("Failed to fetch definition for %s %s.%s: %v", results[idx].Type, results[idx].Schema, results[idx].Name, err)
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
		foreign_keys AS (
			SELECT DISTINCT
				kcu.column_name,
				'constraint ' || 
				'fk_tbl_' || ccu.table_name || '_col_' || kcu.column_name || 
				' references ' || 
				quote_ident(ccu.table_schema) || '.' || quote_ident(ccu.table_name) ||
				CASE
					WHEN rc.delete_rule = 'CASCADE' THEN ' on delete cascade'
					WHEN rc.delete_rule = 'SET NULL' THEN ' on delete set null'
					WHEN rc.delete_rule = 'SET DEFAULT' THEN ' on delete set default'
					WHEN rc.delete_rule = 'RESTRICT' THEN ' on delete restrict'
					ELSE ''
				END as fk_definition,
				tc.constraint_name
			FROM information_schema.table_constraints tc
			JOIN information_schema.key_column_usage kcu
				ON tc.constraint_name = kcu.constraint_name
				AND tc.table_schema = kcu.table_schema
				AND tc.table_name = kcu.table_name
			JOIN information_schema.constraint_column_usage ccu
				ON ccu.constraint_name = tc.constraint_name
				AND ccu.constraint_schema = tc.constraint_schema
			JOIN information_schema.referential_constraints rc
				ON tc.constraint_name = rc.constraint_name
				AND tc.constraint_schema = rc.constraint_schema
			WHERE tc.constraint_type = 'FOREIGN KEY'
			AND tc.table_schema = $1
			AND tc.table_name = $2
		),
		fk_by_column AS (
			SELECT
				column_name,
				string_agg(DISTINCT ' ' || fk_definition, ' ') as all_fk_definitions
			FROM foreign_keys
			GROUP BY column_name
		),
		constraints AS (
			SELECT 
				pg_get_constraintdef(c.oid) as definition
			FROM pg_constraint c
			JOIN pg_namespace n ON n.oid = c.connamespace
			WHERE n.nspname = $1 
			AND c.conrelid::regclass::text = quote_ident($1) || '.' || quote_ident($2)
			AND c.contype != 'f' -- Exclude foreign keys as we handle them separately
		)
		SELECT 
			'CREATE TABLE ' || quote_ident($1) || '.' || quote_ident($2) || ' (' || E'\n' ||
			(SELECT string_agg(
				'    ' || quote_ident(c.column_name) || ' ' || c.data_type || c.size || 
				CASE WHEN c.is_nullable = 'NO' THEN ' NOT NULL' ELSE '' END ||
				CASE WHEN c.column_default IS NOT NULL THEN ' DEFAULT ' || c.column_default ELSE '' END ||
				COALESCE((
					SELECT all_fk_definitions
					FROM fk_by_column fk
					WHERE fk.column_name = c.column_name
				), ''),
				E',\n'
			) FROM columns c) ||
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
			WHERE schema_name = ($1)::text
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

// querySequences queries sequences from the database
func (c *Connector) querySequences(ctx context.Context, schema string, pattern *regexp.Regexp) ([]types.DBObject, error) {
	query := `
		SELECT 
			'sequence' as type,
			sequence_schema as schema,
			sequence_name as name,
			CASE 
				WHEN t.relname IS NOT NULL THEN t.relname 
				ELSE NULL 
			END as table_name
		FROM information_schema.sequences s
		LEFT JOIN (
			SELECT 
				n.nspname as sequence_schema,
				c.relname as sequence_name,
				table_c.relname as relname 
			FROM pg_class c 
			JOIN pg_namespace n ON n.oid = c.relnamespace
			JOIN pg_depend d ON d.objid = c.oid
			JOIN pg_class table_c ON d.refobjid = table_c.oid
			WHERE c.relkind = 'S'
			AND d.deptype = 'a'
			AND d.refclassid = 'pg_class'::regclass
		) t USING(sequence_schema, sequence_name)
		WHERE sequence_schema = ($1)::text
	`
	rows, err := c.db.QueryContext(ctx, query, schema)
	if err != nil {
		return nil, stacktrace.Propagate(err, "Failed to query sequences in schema: %s", schema)
	}
	defer rows.Close()

	var objects []types.DBObject
	for rows.Next() {
		var obj types.DBObject
		var typeStr string
		var tableName sql.NullString
		if err := rows.Scan(&typeStr, &obj.Schema, &obj.Name, &tableName); err != nil {
			return nil, stacktrace.Propagate(err, "Failed to scan sequence row")
		}
		obj.Type = types.ObjectType(typeStr)
		if tableName.Valid {
			obj.TableName = tableName.String
		}
		if pattern.MatchString(obj.Name) {
			objects = append(objects, obj)
		}
	}
	return objects, nil
}

// queryMaterializedViews queries materialized views from the database
func (c *Connector) queryMaterializedViews(ctx context.Context, schema string, pattern *regexp.Regexp) ([]types.DBObject, error) {
	query := `
		SELECT 
			'materialized_view' as type,
			n.nspname as schema,
			c.relname as name
		FROM pg_class c
		JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE c.relkind = 'm'
		AND n.nspname = ($1)::text
	`
	rows, err := c.db.QueryContext(ctx, query, schema)
	if err != nil {
		return nil, stacktrace.Propagate(err, "Failed to query materialized views in schema: %s", schema)
	}
	defer rows.Close()

	var objects []types.DBObject
	for rows.Next() {
		var obj types.DBObject
		var typeStr string
		if err := rows.Scan(&typeStr, &obj.Schema, &obj.Name); err != nil {
			return nil, stacktrace.Propagate(err, "Failed to scan materialized view row")
		}
		obj.Type = types.ObjectType(typeStr)
		if pattern.MatchString(obj.Name) {
			objects = append(objects, obj)
		}
	}
	return objects, nil
}

// queryPolicies queries row-level security policies from the database
func (c *Connector) queryPolicies(ctx context.Context, schema string, pattern *regexp.Regexp) ([]types.DBObject, error) {
	query := `
		SELECT 
			'policy' as type,
			n.nspname as schema,
			pol.polname as name,
			c.relname as table_name
		FROM pg_policy pol
		JOIN pg_class c ON pol.polrelid = c.oid
		JOIN pg_namespace n ON c.relnamespace = n.oid
		WHERE n.nspname = ($1)::text
	`
	rows, err := c.db.QueryContext(ctx, query, schema)
	if err != nil {
		return nil, stacktrace.Propagate(err, "Failed to query policies in schema: %s", schema)
	}
	defer rows.Close()

	var objects []types.DBObject
	for rows.Next() {
		var obj types.DBObject
		var typeStr string
		if err := rows.Scan(&typeStr, &obj.Schema, &obj.Name, &obj.TableName); err != nil {
			return nil, stacktrace.Propagate(err, "Failed to scan policy row")
		}
		obj.Type = types.ObjectType(typeStr)
		if pattern.MatchString(obj.Name) {
			objects = append(objects, obj)
		}
	}
	return objects, nil
}

// queryExtensions queries extensions from the database
func (c *Connector) queryExtensions(ctx context.Context, schema string, pattern *regexp.Regexp) ([]types.DBObject, error) {
	query := `
		SELECT 
			'extension' as type,
			n.nspname as schema,
			e.extname as name
		FROM pg_extension e
		JOIN pg_namespace n ON n.oid = e.extnamespace
		WHERE n.nspname = ($1)::text
	`
	rows, err := c.db.QueryContext(ctx, query, schema)
	if err != nil {
		return nil, stacktrace.Propagate(err, "Failed to query extensions in schema: %s", schema)
	}
	defer rows.Close()

	var objects []types.DBObject
	for rows.Next() {
		var obj types.DBObject
		var typeStr string
		if err := rows.Scan(&typeStr, &obj.Schema, &obj.Name); err != nil {
			return nil, stacktrace.Propagate(err, "Failed to scan extension row")
		}
		obj.Type = types.ObjectType(typeStr)
		if pattern.MatchString(obj.Name) {
			objects = append(objects, obj)
		}
	}
	return objects, nil
}

// queryProcedures queries procedures from the database (PostgreSQL 11+)
func (c *Connector) queryProcedures(ctx context.Context, schema string, pattern *regexp.Regexp) ([]types.DBObject, error) {
	query := `
		SELECT 
			'procedure' as type,
			n.nspname as schema,
			p.proname as name
		FROM pg_proc p
		JOIN pg_namespace n ON n.oid = p.pronamespace
		WHERE n.nspname = ($1)::text
		AND p.prokind = 'p'
	`
	rows, err := c.db.QueryContext(ctx, query, schema)
	if err != nil {
		return nil, stacktrace.Propagate(err, "Failed to query procedures in schema: %s", schema)
	}
	defer rows.Close()

	var objects []types.DBObject
	for rows.Next() {
		var obj types.DBObject
		var typeStr string
		if err := rows.Scan(&typeStr, &obj.Schema, &obj.Name); err != nil {
			return nil, stacktrace.Propagate(err, "Failed to scan procedure row")
		}
		obj.Type = types.ObjectType(typeStr)
		if pattern.MatchString(obj.Name) {
			objects = append(objects, obj)
		}
	}
	return objects, nil
}

// queryPublications queries logical replication publications
func (c *Connector) queryPublications(ctx context.Context, pattern *regexp.Regexp) ([]types.DBObject, error) {
	query := `
		SELECT 
			'publication' as type,
			'postgres' as schema, -- Using 'postgres' as a placeholder for database-level objects
			pubname as name
		FROM pg_publication
	`
	rows, err := c.db.QueryContext(ctx, query)
	if err != nil {
		return nil, stacktrace.Propagate(err, "Failed to query publications")
	}
	defer rows.Close()

	var objects []types.DBObject
	for rows.Next() {
		var obj types.DBObject
		var typeStr string
		if err := rows.Scan(&typeStr, &obj.Schema, &obj.Name); err != nil {
			return nil, stacktrace.Propagate(err, "Failed to scan publication row")
		}
		obj.Type = types.ObjectType(typeStr)
		if pattern.MatchString(obj.Name) {
			objects = append(objects, obj)
		}
	}
	return objects, nil
}

// querySubscriptions queries logical replication subscriptions
func (c *Connector) querySubscriptions(ctx context.Context, pattern *regexp.Regexp) ([]types.DBObject, error) {
	query := `
		SELECT 
			'subscription' as type,
			'postgres' as schema, -- Using 'postgres' as a placeholder for database-level objects
			subname as name
		FROM pg_subscription
	`
	rows, err := c.db.QueryContext(ctx, query)
	if err != nil {
		return nil, stacktrace.Propagate(err, "Failed to query subscriptions")
	}
	defer rows.Close()

	var objects []types.DBObject
	for rows.Next() {
		var obj types.DBObject
		var typeStr string
		if err := rows.Scan(&typeStr, &obj.Schema, &obj.Name); err != nil {
			return nil, stacktrace.Propagate(err, "Failed to scan subscription row")
		}
		obj.Type = types.ObjectType(typeStr)
		if pattern.MatchString(obj.Name) {
			objects = append(objects, obj)
		}
	}
	return objects, nil
}

// queryRules queries rewrite rules from the database
func (c *Connector) queryRules(ctx context.Context, schema string, pattern *regexp.Regexp) ([]types.DBObject, error) {
	query := `
		SELECT 
			'rule' as type,
			n.nspname as schema,
			r.rulename as name,
			c.relname as table_name
		FROM pg_rewrite r
		JOIN pg_class c ON r.ev_class = c.oid
		JOIN pg_namespace n ON c.relnamespace = n.oid
		WHERE n.nspname = ($1)::text
		AND r.rulename != '_RETURN'
	`
	rows, err := c.db.QueryContext(ctx, query, schema)
	if err != nil {
		return nil, stacktrace.Propagate(err, "Failed to query rules in schema: %s", schema)
	}
	defer rows.Close()

	var objects []types.DBObject
	for rows.Next() {
		var obj types.DBObject
		var typeStr string
		if err := rows.Scan(&typeStr, &obj.Schema, &obj.Name, &obj.TableName); err != nil {
			return nil, stacktrace.Propagate(err, "Failed to scan rule row")
		}
		obj.Type = types.ObjectType(typeStr)
		if pattern.MatchString(obj.Name) {
			objects = append(objects, obj)
		}
	}
	return objects, nil
}

// quoteIdentifierIfNeeded quotes an identifier if it contains uppercase letters
// This ensures that PostgreSQL preserves the case of identifiers
func quoteIdentifierIfNeeded(identifier string) string {
	// If the identifier is already quoted, return it as is
	if strings.HasPrefix(identifier, "\"") && strings.HasSuffix(identifier, "\"") {
		return identifier
	}

	// Check if the identifier contains any uppercase letters
	hasUpperCase := false
	for _, r := range identifier {
		if unicode.IsUpper(r) {
			hasUpperCase = true
			break
		}
	}

	// Quote the identifier if it contains uppercase letters
	if hasUpperCase {
		return fmt.Sprintf("\"%s\"", identifier)
	}

	// Return the identifier as is if it's all lowercase
	return identifier
}
