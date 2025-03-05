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
   - Sets up Go environment with version 1.21
   - Builds the main application binary
   - Runs all tests in the internal packages
   - Ensures the application compiles correctly and all tests pass

2. **Lint and Coverage Workflow**: Provides code quality checks and test coverage metrics:
   - Runs golangci-lint to enforce code quality standards
   - Executes tests with race detection enabled
   - Generates code coverage reports
   - Uploads coverage data to Codecov for visualization and tracking

3. **Workflow Triggers**: Both workflows are triggered on:
   - Pushes to main/master branches
   - Pull requests targeting main/master branches

4. **Dependency Caching**: Uses GitHub Actions' caching mechanism to speed up builds by caching Go modules

This CI/CD setup ensures that:
- The codebase always compiles successfully
- All tests pass before merging changes
- Code quality standards are maintained
- Test coverage is tracked over time

The separation of concerns between the two workflows allows for faster feedback on basic build issues while still providing comprehensive quality checks.
