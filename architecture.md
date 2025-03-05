# PGMeta Architecture

## Database Metadata Extraction

PGMeta is designed to extract and represent PostgreSQL database metadata in a structured format. The system provides a way to query and understand the structure of a PostgreSQL database, including tables, views, functions, and their relationships.

### Key Components

1. **Metadata Extraction**: The system uses SQL queries to extract metadata from PostgreSQL system catalogs like `information_schema` and `pg_catalog`.

2. **Object Definition Generation**: For each database object (tables, views, functions, etc.), the system can generate a CREATE statement that represents its definition.

3. **Relationship Handling**: The system now includes foreign key relationship information in table definitions, making it easier to understand table relationships at a glance. This is done by:
   - Querying `information_schema.table_constraints`, `information_schema.key_column_usage`, `information_schema.constraint_column_usage`, and `information_schema.referential_constraints`
   - Generating constraint names in a standardized format (`fk_tbl_[referenced_table]_col_[column_name]`)
   - Including schema qualification in references
   - Preserving ON DELETE behavior (CASCADE, SET NULL, SET DEFAULT, RESTRICT)
   - Handling multiple foreign keys on a single column by aggregating them together
   - Ensuring uniqueness of constraints using DISTINCT in both the selection and aggregation phases

4. **Concurrent Processing**: The system supports concurrent processing of database objects to improve performance when dealing with large databases.

5. **Identifier Quoting**: The system properly quotes identifiers in SQL queries to preserve case sensitivity:
   - Uses PostgreSQL's `quote_ident` function to automatically quote identifiers when needed
   - Ensures that identifiers with uppercase letters are properly quoted to prevent them from being coerced to lowercase in SQL
   - Maintains aesthetic consistency by not quoting identifiers that don't need it (all lowercase)
   - This approach ensures that database objects with mixed-case names are correctly referenced and created

### Design Patterns

1. **Connector Pattern**: The `Connector` struct encapsulates database connection logic and provides methods for querying database metadata.

2. **Builder Pattern**: SQL queries are built using string templates, allowing for complex queries to be constructed in a readable way.

3. **Mock Testing**: The system uses mock objects for testing, allowing tests to run without an actual database connection.

### SQL Query Techniques

1. **Common Table Expressions (CTEs)**: The system makes extensive use of CTEs to organize complex queries into logical components, making them easier to understand and maintain.

2. **String Aggregation**: For cases where multiple rows need to be combined (like multiple foreign keys on a single column), the system uses PostgreSQL's `string_agg` function with the DISTINCT modifier to concatenate unique results and avoid duplication.

3. **Subquery Optimization**: To avoid the "more than one row returned by a subquery used as an expression" error, the system pre-aggregates data that might return multiple rows before using it in a subquery expression.

4. **Duplicate Prevention**: The system uses multiple layers of duplicate prevention (DISTINCT in both the selection and aggregation phases) to ensure that metadata is clean and not redundant, which is especially important for complex database schemas with many relationships.

5. **Proper Identifier Quoting**: The system uses PostgreSQL's built-in `quote_ident` function to ensure that identifiers are properly quoted only when necessary, preserving case sensitivity while maintaining readability.

This architecture allows PGMeta to provide rich metadata about PostgreSQL databases, making it useful for database documentation, schema visualization, and other database management tools.

### Testing Architecture

1. **Interface-Based Testing**: The system uses Go interfaces to define the contract between components, allowing for easy mocking in tests. The `dbConnector` interface defines the methods required for database interaction, which can be implemented by both real database connectors and mock objects.

2. **Mock Implementations**: Several mock implementations are used for testing:
   - `mockConnector`: A basic mock that can be configured to either succeed or fail all operations
   - `selectiveFailConnector`: A more sophisticated mock that can be configured to fail specific operations based on object names

3. **Failure Simulation**: The testing framework includes mechanisms to simulate different types of failures:
   - Complete failures (all operations fail)
   - Selective failures (specific objects fail)
   - This allows testing of error handling and recovery paths

4. **Error Handling Modes**: The system supports two error handling modes:
   - Fail fast: Stop processing on the first error
   - Continue with warnings: Log errors but continue processing other objects
   - Tests verify both modes work correctly

5. **Struct Embedding for Mocks**: The system uses Go's struct embedding to extend mock implementations, allowing for selective override of specific methods while inheriting others. This is particularly useful for the `selectiveFailConnector` which extends the base `mockConnector`.

6. **Temporary Directory Management**: Tests create temporary directories for output and clean them up after test completion, ensuring tests don't leave artifacts behind.

7. **Concurrent Operation Testing**: The system includes tests specifically designed to verify that concurrent operations work correctly, with proper synchronization to prevent race conditions.

These testing patterns ensure that PGMeta remains robust and reliable, even when encountering errors or unexpected conditions in real-world database environments.

### CI/CD Architecture

The project uses GitHub Actions for continuous integration and delivery, ensuring code quality and reliability:

1. **Build and Test Workflow**: Automatically builds the application and runs tests on every push and pull request:
   - Sets up Go environment with version 1.24.0
   - Builds the main application binary
   - Runs all tests in the internal packages
   - Ensures the application compiles correctly and all tests pass

2. **Lint and Coverage Workflow**: Provides code quality checks and test coverage metrics:
   - Runs golangci-lint to enforce code quality standards
   - Executes tests with race detection enabled
   - Generates code coverage reports
   - Uploads coverage data to Codecov for visualization and tracking

3. **Release Workflow**: Automates the release process when a new version tag is pushed:
   - Triggered only when a tag matching the pattern `v*` is pushed
   - Performs all quality checks from the build/test and lint workflows
   - Uses GoReleaser to build cross-platform binaries
   - Creates GitHub Releases with built artifacts
   - Ensures releases follow the same quality standards as regular code changes

4. **Workflow Triggers**: The workflows are triggered on different events:
   - Build/Test and Lint/Coverage: Pushes to main/master branches and pull requests
   - Release: Pushes of tags matching the pattern `v*`

5. **Dependency Caching**: Uses GitHub Actions' caching mechanism to speed up builds by caching Go modules

6. **Quality Gates**: Each workflow serves as a quality gate:
   - Code cannot be merged if it fails the Build/Test or Lint/Coverage workflows
   - Releases cannot be published if they fail the Release workflow
   - This ensures that only high-quality code reaches users

This CI/CD setup ensures that:
- The codebase always compiles successfully
- All tests pass before merging changes
- Code quality standards are maintained
- Test coverage is tracked over time
- Releases follow a consistent, automated process
- Users receive well-tested, properly built binaries

The separation of concerns between the workflows allows for faster feedback on basic build issues while still providing comprehensive quality checks. The release workflow builds upon the same quality foundations to ensure that released artifacts meet the same high standards as the codebase itself.

### Error Handling Patterns

The project follows several error handling patterns to ensure robustness:

1. **Explicit Error Checking**: All function calls that can return errors are explicitly checked, including:
   - Logger output operations
   - File operations
   - Command flag operations

2. **Error Propagation with Context**: When errors occur, they are propagated up the call stack with additional context using the stacktrace package:
   - Uses format strings with proper type safety
   - Adds meaningful context to errors to aid in debugging

3. **Graceful Degradation**: The system is designed to handle errors gracefully:
   - Logging errors when they occur
   - Providing fallback mechanisms when possible
   - Allowing operations to continue when appropriate (configurable via continueOnError flag)

4. **Testing Error Paths**: The testing framework includes specific tests for error conditions to ensure proper handling.

These error handling patterns ensure that the application behaves predictably even when encountering unexpected conditions, making it more reliable in production environments.

### Versioning Architecture

The project implements a clean, maintainable approach to version management:

1. **Version Package**: A dedicated package (`internal/version`) encapsulates all version-related functionality:
   - Exports a `Version` variable that holds the current version string
   - Provides a `GetVersion()` function to retrieve the version
   - Isolates version management from the rest of the codebase

2. **Build-Time Version Injection**: The version is injected at build time rather than hardcoded:
   - Uses Go's `-ldflags` mechanism to set the version variable
   - The GoReleaser configuration includes the appropriate ldflags setting:
     ```
     ldflags:
       - -s -w -X github.com/skamensky/pgmeta/internal/version.Version={{.Version}}
     ```
   - This approach ensures the version is always accurate without requiring code changes

3. **Version Command**: The CLI includes a dedicated `version` command:
   - Implemented as a Cobra command in the main application
   - Displays the current version when invoked with `pgmeta version`
   - Helps users identify which version they're running

4. **Default Development Version**: During development, the version defaults to "dev":
   - The `Version` variable is initialized to "dev" in the source code
   - This ensures that development builds are clearly identifiable
   - Only official releases receive proper version numbers through the build process

5. **Version Accessibility**: The version information is:
   - Available to users through the CLI
   - Accessible programmatically for other components that need version information
   - Consistent across all parts of the application

This versioning architecture ensures that:
- Version information is managed in a single location
- The build process automatically sets the correct version
- Users can easily determine which version they're using
- Development and release builds are clearly distinguishable
- Version updates don't require code changes

The clean separation of version management from other concerns makes the codebase more maintainable and reduces the risk of version inconsistencies.

### Release Architecture

The project implements a robust release process that ensures high-quality, consistent releases across multiple platforms:

1. **Semantic Versioning**: The project follows semantic versioning (MAJOR.MINOR.PATCH) to clearly communicate the nature of changes:
   - MAJOR: Breaking changes that require updates to client code
   - MINOR: New features that maintain backward compatibility
   - PATCH: Bug fixes and minor improvements that maintain backward compatibility

2. **Version Management**: Version information is managed through:
   - A dedicated `version` package in `internal/version`
   - Version injection at build time using ldflags
   - A CLI command to display the current version

3. **Release Workflow**: The release process is automated using GitHub Actions:
   - Triggered by pushing a tag with the format `v*` (e.g., `v1.0.0`)
   - Performs code formatting with `go fmt`
   - Runs linting checks with `golangci-lint`
   - Executes tests with race detection and coverage reporting
   - Builds binaries for multiple platforms (Linux, macOS, Windows) and architectures (amd64, arm64)
   - Creates a GitHub Release with the binaries attached
   - Generates release notes based on commit history

4. **Cross-Platform Building**: The project uses GoReleaser to:
   - Build binaries for multiple operating systems and architectures
   - Configure build flags and environment variables consistently
   - Create distribution archives with appropriate formats for each platform
   - Generate checksums for verification
   - Filter the changelog to exclude irrelevant commits

5. **Pre-Release Quality Assurance**: Before creating a release, the code undergoes several quality checks:
   - Code formatting verification
   - Linting to catch common issues
   - Test execution to ensure functionality
   - Race condition detection to prevent concurrency issues
   - Test coverage measurement to maintain or improve code quality

6. **Release Artifacts**: Each release produces:
   - Platform-specific binaries (Linux, macOS, Windows)
   - Architecture-specific builds (amd64, arm64)
   - Compressed archives (.tar.gz for Unix-like systems, .zip for Windows)
   - Checksums for verifying download integrity
   - Automatically generated release notes

7. **Quality Enforcement Tools**:
   - A pre-release check script (`scripts/pre-release-check.sh`) to automate quality verification
   - A release checklist (`RELEASE_CHECKLIST.md`) to ensure all steps are completed
   - Automated checks in the CI/CD pipeline to prevent releases with quality issues

This release architecture ensures that:
- All releases meet the project's quality standards
- The release process is consistent and repeatable
- Users receive well-tested, properly built binaries for their platform
- Version information is clear and follows industry standards
- The release history is well-documented and easy to understand

The combination of automated tools and clear processes makes releasing new versions straightforward while maintaining high quality standards.
