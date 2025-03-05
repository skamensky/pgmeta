# pgmeta: PostgreSQL Schema Extraction Tool

pgmeta is a lightweight, powerful utility for extracting PostgreSQL database schemas into a structured format that's easy to version control, analyze, and incorporate into development workflows.

## Development

- I edited around a dozen lines of code in this code base.
- The initial code was generated via an english conversation with Claude Sonnet 3.5 inside of Cursor.
- Afterward, I used Claude Code with Claude Sonnet 3.7 to do a review and generate tests.

## Documentation

For more detailed information about the project, please refer to the following documentation:

- [Architecture Overview](./docs/architecture.md) - High-level design and architecture of pgmeta
- [Release Checklist](./docs/release_checklist.md) - Process for creating new releases

## Overview

Working with complex PostgreSQL databases often requires keeping track of database schema objects like tables, functions, constraints, and triggers. pgmeta solves this challenge by connecting to your PostgreSQL database and exporting all schema objects into a well-organized directory structure of SQL files.

This is particularly useful for:

- Documentation: See your entire database structure at a glance
- Version control: Track schema changes over time 
- Analysis: Easily review database objects and their relationships
- Migration planning: Compare schemas between environments
- LLM context: Provide database schema context to your AI assistants for better code generation and database interactions

## Command Line Interface

```
$ pgmeta --help
PostgreSQL metadata extraction tool

Usage:
  pgmeta [command]

Available Commands:
  completion  Generate the autocompletion script for the specified shell
  connection  Manage database connections
  help        Help about any command
  export      Export database metadata

Flags:
      --debug   Enable debug mode with stack traces
  -h, --help    help for pgmeta

Use "pgmeta [command] --help" for more information about a command.
```

### Connection Management

```
$ pgmeta connection --help
Manage database connections

Usage:
  pgmeta connection [command]

Available Commands:
  create       Create a new connection
  delete       Delete a connection
  list         List all connections
  make-default Set a connection as default

Flags:
  -h, --help   help for connection

Global Flags:
      --debug   Enable debug mode with stack traces

Use "pgmeta connection [command] --help" for more information about a command.
```

### Export Metadata

```
$ pgmeta export --help
Export database metadata

Usage:
  pgmeta export [flags]

Flags:
      --connection string   Connection name (optional)
  -h, --help                help for export
      --on-error string     Error handling behavior: 'fail' (default) or 'warn' (default "fail")
      --output string       Output directory for generated files (default "./pgmeta-output")
      --query string        Regex pattern to match object names (optional, 'ALL' fetches everything) (default "ALL")
      --schema string       Schema name (optional) (default "public")
      --types string        Comma-separated list of object types. Valid types: ALL, table, view, function, trigger, index, constraint (default "ALL")

Global Flags:
      --debug   Enable debug mode with stack traces
```

## Releases

pgmeta uses GitHub Releases to distribute pre-built binaries for multiple platforms. You can find the latest release on the [GitHub Releases page](https://github.com/skamensky/pgmeta/releases).

### Creating a New Release

To create a new release:

1. Ensure your code meets quality standards:
   - Run `go fmt ./...` to format all Go files
   - Run `golangci-lint run ./...` to check for linting issues
   - Run `go test ./internal/...` to verify all tests pass

2. Update your code and commit all changes

3. Create and push a new tag with a semantic version:
   ```bash
   git tag -a v0.1.0 -m "First release"
   git push origin v0.1.0
   ```

4. GitHub Actions will automatically:
   - Format code with `go fmt`
   - Run linting checks with `golangci-lint`
   - Run tests with race detection and coverage reporting
   - Build binaries for multiple platforms (Linux, macOS, Windows)
   - Create a GitHub Release with the binaries attached
   - Generate release notes based on commit history

### Release Quality Standards

Each release automatically goes through several quality checks:

1. **Code Formatting**: All Go files are formatted using `go fmt` to ensure consistent style.
2. **Linting**: `golangci-lint` checks for common issues and enforces code quality standards.
3. **Testing**: All tests are run with race detection enabled to catch concurrency issues.
4. **Coverage**: Test coverage is measured to ensure code is properly tested.

These checks help maintain the high quality and reliability of pgmeta releases.

### Installing from Releases

You can download the pre-built binary for your platform from the [Releases page](https://github.com/skamensky/pgmeta/releases), or use the following commands:

#### Linux (amd64)
```bash
curl -L https://github.com/skamensky/pgmeta/releases/latest/download/pgmeta_Linux_x86_64.tar.gz | tar xz
sudo mv pgmeta /usr/local/bin/
```

#### macOS (amd64)
```bash
curl -L https://github.com/skamensky/pgmeta/releases/latest/download/pgmeta_Darwin_x86_64.tar.gz | tar xz
sudo mv pgmeta /usr/local/bin/
```

#### Windows (amd64)
Download the ZIP file from the Releases page and extract it to a location in your PATH.

## Installation

Simply download the compiled binary for your platform or build from source:

```bash
# Clone the repository
git clone https://github.com/skamensky/pgmeta.git

# Build the binary
cd pgmeta
go build -o pgmeta ./cmd/pgmeta
```

## Getting Started

### Managing Connections

Before using pgmeta, you'll need to set up a connection to your PostgreSQL database:

```bash
# Create a new connection
pgmeta connection create --name dev --url "postgres://user:password@localhost:5432/database" --make-default

# List configured connections
pgmeta connection list

# Change default connection
pgmeta connection make-default --name prod

# Remove a connection
pgmeta connection delete --name old-db
```

### Extracting Schema Objects

Once you've configured a connection, you can extract database objects:

```bash
# Extract all database objects
pgmeta export

# Extract specific object types
pgmeta export --types table,function

# Extract objects matching a name pattern (regex)
pgmeta export --query "user.*"

# Extract from a specific schema
pgmeta export --schema public

# Extract from multiple schemas
pgmeta export --schema public,customers,orders

# Extract from all schemas
pgmeta export --schema ALL

# Specify output directory
pgmeta export --output ./my-db-schema

# Continue exporting despite errors
pgmeta export --on-error warn
```

## Supported Object Types

pgmeta can extract the following PostgreSQL object types:

- `table`: Database tables with their column definitions
- `view`: Database views and their queries
- `function`: User-defined functions
- `aggregate`: User-defined aggregate functions
- `trigger`: Table triggers
- `index`: Table indexes
- `constraint`: Table constraints (primary keys, foreign keys, unique constraints, check constraints)
- `sequence`: Database sequences (stored at the table level when owned by a table column)
- `materialized_view`: Materialized views with their queries (stored at the schema level)
- `policy`: Row-level security policies (stored at the table level)
- `extension`: PostgreSQL extensions (stored at the schema level)
- `procedure`: Stored procedures (PostgreSQL 11+ only, stored at the schema level)
- `publication`: Logical replication publications (stored at the database level)
- `subscription`: Logical replication subscriptions (stored at the database level)
- `rule`: Query rewrite rules (stored at the table level or in the schema's 'rules' directory)

> **Note on PostgreSQL Version Compatibility**: Some object types like `sequence`, `policy`, `publication`, and `subscription` may have limited support on older PostgreSQL versions (prior to 10). When exporting from older PostgreSQL servers, use the `--on-error warn` flag to continue despite errors with these newer object types.

### Default Values

- **Types**: When `--types` is not specified or set to `ALL`, pgmeta extracts all object types
- **Query**: When `--query` is not specified or set to `ALL`, pgmeta extracts all objects (uses `.*` regex pattern)
- **Schema**: When `--schema` is not specified, pgmeta defaults to the `public` schema. Use a comma-separated list to specify multiple schemas, or use `ALL` to extract from all schemas.
- **Output**: When `--output` is not specified, pgmeta uses `./pgmeta-output` as the output directory
- **Connection**: When `--connection` is not specified, pgmeta uses the default connection
- **On-Error**: When `--on-error` is not specified, pgmeta defaults to `warn`, which continues extraction despite errors. Use `fail` to stop when any error occurs. Note: For older PostgreSQL versions (prior to 10), use `warn` as some newer object types may not be fully supported.

## Output Structure

pgmeta organizes the output into a directory structure that mirrors your database schema:

```
pgmeta-output/
├── public/                  # Schema name
│   ├── functions/
│   │   ├── function1.sql
│   │   └── function2.sql
│   ├── procedures/
│   │   └── procedure1.sql
│   ├── views/
│   │   └── view1.sql
│   ├── materialized_views/
│   │   └── matview1.sql
│   ├── extensions/
│   │   └── pgcrypto.sql
│   ├── rules/
│   │   └── standalone_rule.sql
│   └── tables/
│       ├── table1/
│       │   ├── table.sql
│       │   ├── constraints/
│       │   │   ├── table1_pkey.sql
│       │   │   └── fk_table1_col_ref.sql
│       │   ├── indexes/
│       │   │   └── table1_idx.sql
│       │   ├── triggers/
│       │   │   └── table1_audit_trigger.sql
│       │   ├── sequences/
│       │   │   └── table1_id_seq.sql
│       │   ├── policies/
│       │   │   └── table1_rls_policy.sql
│       │   └── rules/
│       │       └── table1_insert_rule.sql
│       └── table2/
│           └── ...
├── app/                     # Another schema
│   ├── functions/
│   │   └── app_function.sql
│   └── tables/
│       └── ...
├── reporting/               # Yet another schema
│   └── views/
│       └── sales_summary.sql
└── postgres/                # Database-level objects
    ├── publications/
    │   └── pub_orders.sql
    └── subscriptions/
        └── sub_remote_data.sql
```

This structure makes it easy to navigate and understand the relationships between different database objects across multiple schemas.

## Why Use pgmeta?

Unlike other database schema tools, pgmeta:

- Is source code centric
- Enables you to provide database context to AI assistants for smarter code generation (my main use case)
- Creates a structured, hierarchical representation of your database
- Groups related objects together (tables with their indexes, constraints, and triggers)
- Produces clean, readable SQL files that are easy to version control
- Provides a simple command-line interface that's easy to automate
- Safely manages database connections with secure credential handling

## Use Cases

### Add Database Context to LLMs

One powerful use of pgmeta is to provide database schema context to large language models (LLMs). By exporting your schema and including it in your prompts, you can help AI assistants like Claude better understand your database structure, write more accurate SQL queries, and provide better assistance with database-related tasks.

```bash
# Extract database schema for AI context
pgmeta export --output ./db-context
```

Then include the generated SQL files in your prompts to the AI assistant.

## License

pgmeta is open source software licensed under the MIT license.