package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/palantir/stacktrace"
	"github.com/skamensky/pgmeta/internal/config"
	"github.com/skamensky/pgmeta/internal/log"
	"github.com/skamensky/pgmeta/internal/metadata"
	"github.com/skamensky/pgmeta/internal/metadata/types"
	"github.com/spf13/cobra"
)

var debugMode bool

func main() {
	if err := rootCmd.Execute(); err != nil {
		if debugMode {
			// In debug mode, show full stacktrace
			fmt.Fprintln(os.Stderr, err)
		} else {
			// In normal mode, only show the message without stacktrace
			msg := stacktrace.RootCause(err).Error()
			msg = strings.TrimPrefix(msg, "Error: ")
			fmt.Fprintln(os.Stderr, "Error:", msg)
		}
		os.Exit(1)
	}
}

var rootCmd = &cobra.Command{
	Use:          "pgmeta",
	Short:        "PostgreSQL metadata extraction tool",
	SilenceUsage: true,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		// Configure logging based on debug flag
		if debugMode {
			log.EnableDebugMode()
			log.Debug("Debug mode enabled")
		}
	},
}

func init() {
	rootCmd.PersistentFlags().BoolVar(&debugMode, "debug", false, "Enable debug mode with stack traces")
	connectionCmd := &cobra.Command{
		Use:   "connection",
		Short: "Manage database connections",
	}
	rootCmd.AddCommand(connectionCmd)

	createCmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new connection",
		RunE:  runCreateConnection,
	}
	createCmd.Flags().String("name", "", "Connection name (required)")
	createCmd.Flags().String("url", "", "Database URL (required)")
	createCmd.Flags().Bool("make-default", false, "Set as default connection")
	createCmd.MarkFlagRequired("name")
	createCmd.MarkFlagRequired("url")

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List all connections",
		RunE:  runListConnections,
	}

	deleteCmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete a connection",
		RunE:  runDeleteConnection,
	}
	deleteCmd.Flags().String("name", "", "Connection name (required)")
	deleteCmd.MarkFlagRequired("name")

	makeDefaultCmd := &cobra.Command{
		Use:   "make-default",
		Short: "Set a connection as default",
		RunE:  runMakeDefaultConnection,
	}
	makeDefaultCmd.Flags().String("name", "", "Connection name (required)")
	makeDefaultCmd.MarkFlagRequired("name")

	connectionCmd.AddCommand(createCmd, listCmd, deleteCmd, makeDefaultCmd)

	exportCmd := &cobra.Command{
		Use:   "export",
		Short: "Export database metadata",
		RunE:  runExport,
	}
	exportCmd.Flags().String("query", "ALL", "Regex pattern to match object names (optional, 'ALL' fetches everything)")
	exportCmd.Flags().String("types", "ALL", "Comma-separated list of object types. Valid types: ALL, table, view, function, aggregate, trigger, index, constraint, sequence, materialized_view, policy, extension, procedure, publication, subscription, rule")
	exportCmd.Flags().String("connection", "", "Connection name (optional). Defaults to the default connection ")
	exportCmd.Flags().String("schema", "public", "Comma-separated list of schema names or 'ALL' to export all schemas (optional)")
	exportCmd.Flags().String("output", "./pgmeta-output", "Output directory for generated files")
	exportCmd.Flags().String("on-error", "warn", "Error handling behavior: 'warn' (default) or 'fail' (Use 'warn' for older PostgreSQL versions)")

	rootCmd.AddCommand(exportCmd)
}

func runCreateConnection(cmd *cobra.Command, args []string) error {
	name, _ := cmd.Flags().GetString("name")
	url, _ := cmd.Flags().GetString("url")
	makeDefault, _ := cmd.Flags().GetBool("make-default")

	log.Debug("Creating connection %s with URL %s (default: %v)", name, url, makeDefault)

	cfg, err := config.LoadConfig()
	if err != nil {
		return stacktrace.Propagate(err, "Failed to load config")
	}

	if err := cfg.AddConnection(name, url, makeDefault); err != nil {
		return stacktrace.Propagate(err, "Failed to add connection %s", name)
	}

	fmt.Printf("Added new connection: %s\n", name)
	return nil
}

func runListConnections(cmd *cobra.Command, args []string) error {
	log.Debug("Listing connections")

	cfg, err := config.LoadConfig()
	if err != nil {
		return stacktrace.Propagate(err, "Failed to load config")
	}

	if len(cfg.Connections) == 0 {
		fmt.Println("No connections configured")
		return nil
	}

	fmt.Println("Configured connections:")
	for _, conn := range cfg.Connections {
		defaultMark := " "
		if conn.IsDefault {
			defaultMark = "*"
		}
		fmt.Printf("%s %s: %s\n", defaultMark, conn.Name, conn.URL)
	}
	return nil
}

func runDeleteConnection(cmd *cobra.Command, args []string) error {
	name, _ := cmd.Flags().GetString("name")

	log.Debug("Deleting connection: %s", name)

	cfg, err := config.LoadConfig()
	if err != nil {
		return stacktrace.Propagate(err, "Failed to load config")
	}

	if err := cfg.DeleteConnection(name); err != nil {
		return stacktrace.Propagate(err, "Failed to delete connection")
	}

	fmt.Printf("Deleted connection: %s\n", name)
	return nil
}

func runMakeDefaultConnection(cmd *cobra.Command, args []string) error {
	name, _ := cmd.Flags().GetString("name")

	log.Debug("Setting %s as default connection", name)

	cfg, err := config.LoadConfig()
	if err != nil {
		return stacktrace.Propagate(err, "Failed to load config")
	}

	if err := cfg.SetDefaultConnection(name); err != nil {
		return stacktrace.Propagate(err, "Failed to set default connection")
	}

	fmt.Printf("Set %s as default connection\n", name)
	return nil
}

func runExport(cmd *cobra.Command, args []string) error {
	query, _ := cmd.Flags().GetString("query")
	typesList, _ := cmd.Flags().GetString("types")
	connName, _ := cmd.Flags().GetString("connection")
	schemasList, _ := cmd.Flags().GetString("schema")
	outputDir, _ := cmd.Flags().GetString("output")
	onErrorOption, _ := cmd.Flags().GetString("on-error")

	// Validate on-error option
	if onErrorOption != "fail" && onErrorOption != "warn" {
		return stacktrace.NewError("Invalid on-error option: %s. Valid options are: warn, fail", onErrorOption)
	}

	log.Info("Exporting database objects with pattern %s, types %s, schemas %s, on-error: %s",
		query, typesList, schemasList, onErrorOption)

	// Create output directory if it doesn't exist
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return stacktrace.Propagate(err, "Failed to create output directory: %s", outputDir)
	}

	cfg, err := config.LoadConfig()
	if err != nil {
		return stacktrace.Propagate(err, "Failed to load config")
	}

	var connectionURL string
	if connName != "" {
		conn := cfg.GetConnection(connName)
		if conn == nil {
			return stacktrace.NewError("Connection not found: %s", connName)
		}
		connectionURL = conn.URL
		log.Debug("Using specified connection: %s", connName)
	} else {
		conn := cfg.GetDefaultConnection()
		if conn == nil {
			return stacktrace.NewError("No connection specified and no default connection found")
		}
		connectionURL = conn.URL
		log.Debug("Using default connection: %s", conn.Name)
	}

	fetcher, err := metadata.NewFetcher(connectionURL)
	if err != nil {
		return stacktrace.Propagate(err, "Failed to initialize metadata fetcher")
	}
	defer fetcher.Close()

	var objectTypes []types.ObjectType
	if typesList == "ALL" {
		objectTypes = []types.ObjectType{} // Empty slice means all types in our implementation
		log.Debug("Querying all object types")
	} else {
		// Parse comma-separated types
		for _, t := range strings.Split(typesList, ",") {
			objType := types.ObjectType(strings.TrimSpace(t))
			if !metadata.IsValidType(objType) {
				return stacktrace.NewError("Invalid object type: %s. Valid types are: ALL, table, view, function, trigger, index, constraint", t)
			}
			objectTypes = append(objectTypes, objType)
		}
		log.Debug("Querying specific object types: %v", objectTypes)
	}

	// Use a special regex that matches everything if query is "ALL"
	nameRegex := query
	if query == "ALL" {
		nameRegex = ".*" // Regex that matches everything
		log.Debug("Using wildcard regex pattern")
	} else {
		log.Debug("Using regex pattern: %s", nameRegex)
	}

	var schemas []string
	// Special handling for "ALL" to fetch all schemas
	if schemasList == "ALL" {
		allSchemas, err := fetcher.GetAllSchemas()
		if err != nil {
			return stacktrace.Propagate(err, "Failed to fetch all schemas")
		}
		schemas = allSchemas
		log.Info("Fetching objects from all schemas: %v", schemas)
	} else {
		// Parse comma-separated schemas
		for _, s := range strings.Split(schemasList, ",") {
			schemas = append(schemas, strings.TrimSpace(s))
		}
	}

	objects, err := fetcher.QueryObjects(types.QueryOptions{
		Types:     objectTypes,
		Schemas:   schemas,
		NameRegex: nameRegex,
	})
	if err != nil {
		return stacktrace.Propagate(err, "Failed to query objects")
	}

	log.Info("Found %d objects", len(objects))
	if len(objects) > 0 {
		fmt.Println("Found objects:")
		for i, obj := range objects {
			fmt.Printf("%d. [%s] %s.%s\n", i+1, obj.Type, obj.Schema, obj.Name)
		}
	} else {
		fmt.Println("No objects found matching the criteria")
		return nil
	}

	continueOnError := onErrorOption == "warn"
	if err := fetcher.SaveObjects(objects, outputDir, continueOnError); err != nil {
		return stacktrace.Propagate(err, "Failed to save objects")
	}

	fmt.Printf("Successfully saved objects to %s\n", outputDir)
	return nil
}
