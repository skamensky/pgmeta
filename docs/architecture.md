# PGMeta Architecture

## Overview

PGMeta is a PostgreSQL schema extraction tool designed to extract and represent database metadata in a structured format. It provides a way to query and understand the structure of a PostgreSQL database, including tables, views, functions, and their relationships.

## Core Capabilities

1. **Metadata Extraction**: Extracts metadata from PostgreSQL system catalogs
2. **Object Definition Generation**: Generates CREATE statements for database objects
3. **Relationship Handling**: Includes foreign key relationship information in table definitions
4. **Concurrent Processing**: Supports concurrent processing for improved performance
5. **Multi-Schema Support**: Handles multiple schemas simultaneously
6. **Error Handling Options**: Configurable behavior when errors occur (fail or warn)
7. **Backward Compatibility**: Works with older PostgreSQL versions with fallback methods

## Supported PostgreSQL Objects

- Tables and their columns
- Views
- Materialized Views
- Functions
- Procedures
- Sequences
- Policies
- Extensions
- Publications/Subscriptions
- Rules
- Foreign key relationships

## Architecture Components

### Code Organization

The codebase is organized into several key packages:

1. **Metadata Package**:
   - `types`: Core data structures and type definitions
   - `db`: Database connection and query operations
   - `export`: File export operations
   - `fetcher`: Facade for the above components

2. **Log Package**: Standardized logging interface

3. **Config Package**: Connection management and configuration

4. **CLI Commands**: Interface for user interaction

### Design Patterns

1. **Connector Pattern**: Encapsulates database connection logic
2. **Builder Pattern**: Constructs complex SQL queries using templates
3. **Worker Pool Pattern**: Manages concurrent file operations
4. **Semaphore Pattern**: Controls database connection usage
5. **Mock Testing**: Uses interfaces for testability without database connections

### Concurrency Model

1. **Concurrent File Output**:
   - Worker pool for file operations
   - Thread-safe directory creation
   - Configurable concurrency level

2. **Concurrent Database Operations**:
   - Parallel fetching of object definitions
   - Connection pooling with semaphores
   - Thread-safe processing of results

### Error Handling Patterns

1. **Explicit Error Checking**: All function calls that can return errors are explicitly checked
2. **Error Propagation with Context**: Errors are propagated with additional context
3. **Graceful Degradation**: Configurable continue-on-error functionality
4. **Error Recovery**: Handles partial failures during export

### Testing Architecture

1. **Interface-Based Testing**: Uses Go interfaces for component contracts
2. **Mock Implementations**: Simulates database behavior for testing
3. **Failure Simulation**: Tests error handling and recovery paths
4. **Concurrent Operation Testing**: Verifies thread safety

### CI/CD Architecture

The project uses GitHub Actions for continuous integration and delivery:

1. **Build and Test Workflow**: Automatically builds and tests on every push and PR
2. **Lint and Coverage Workflow**: Enforces code quality standards
3. **Release Workflow**: Automates the release process for new version tags
4. **Quality Gates**: Prevents merging or releasing code that doesn't meet standards

### Versioning Architecture

1. **Version Package**: Encapsulates version-related functionality
2. **Build-Time Version Injection**: Sets version at build time via ldflags
3. **Semantic Versioning**: Follows MAJOR.MINOR.PATCH convention
4. **Version Command**: CLI command to display current version

### Release Process

1. **Automated Workflow**: Triggered by version tags
2. **Cross-Platform Building**: Creates binaries for multiple platforms
3. **Pre-Release Quality Assurance**: Runs tests and quality checks
4. **Release Artifacts**: Generates platform-specific binaries and checksums
5. **Quality Enforcement**: Uses pre-release checks and checklists

This architecture ensures PGMeta is robust, maintainable, and provides high-quality PostgreSQL schema extraction capabilities. 