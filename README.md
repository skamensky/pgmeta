# pgmeta: PostgreSQL Schema Extraction Tool

pgmeta is a lightweight, powerful utility for extracting PostgreSQL database schemas into a structured format that's easy to version control, analyze, and incorporate into development workflows.

## Development

I didn't write a _single_ line of code of this code base.
The initial code was generated via an english conversation with Claude Sonnet 3.5 inside of Cursor.
Afterward, I used Claude Code with Claude Sonnet 3.7 to do a review and generate tests.
See [claude_code.md](./claude_code.md).


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

## Installation

Simply download the compiled binary for your platform or build from source:

```bash
# Clone the repository
git clone https://github.com/shkamensky/pgmeta.git

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
- `function`: User-defined functions and stored procedures
- `trigger`: Table triggers
- `index`: Table indexes
- `constraint`: Table constraints (primary keys, foreign keys, unique constraints, check constraints)

### Default Values

- **Types**: When `--types` is not specified or set to `ALL`, pgmeta extracts all object types
- **Query**: When `--query` is not specified or set to `ALL`, pgmeta extracts all objects (uses `.*` regex pattern)
- **Schema**: When `--schema` is not specified, pgmeta defaults to the `public` schema. Use a comma-separated list to specify multiple schemas, or use `ALL` to extract from all schemas.
- **Output**: When `--output` is not specified, pgmeta uses `./pgmeta-output` as the output directory
- **Connection**: When `--connection` is not specified, pgmeta uses the default connection
- **On-Error**: When `--on-error` is not specified, pgmeta defaults to `fail`, which stops extraction when any error occurs. Use `warn` to continue despite errors.

## Output Structure

pgmeta organizes the output into a directory structure that mirrors your database schema:

```
pgmeta-output/
├── public/                  # Schema name
│   ├── functions/
│   │   ├── function1.sql
│   │   └── function2.sql
│   ├── views/
│   │   └── view1.sql
│   └── tables/
│       ├── table1/
│       │   ├── table.sql
│       │   ├── constraints/
│       │   │   ├── table1_pkey.sql
│       │   │   └── fk_table1_col_ref.sql
│       │   ├── indexes/
│       │   │   └── table1_idx.sql
│       │   └── triggers/
│       │       └── table1_audit_trigger.sql
│       └── table2/
│           └── ...
├── app/                     # Another schema
│   ├── functions/
│   │   └── app_function.sql
│   └── tables/
│       └── ...
└── reporting/               # Yet another schema
    └── views/
        └── sales_summary.sql
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