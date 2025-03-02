package metadata

import (
	"context"

	"github.com/skamensky/pgmeta/internal/log"
	"github.com/skamensky/pgmeta/internal/metadata/db"
	"github.com/skamensky/pgmeta/internal/metadata/export"
	"github.com/skamensky/pgmeta/internal/metadata/types"
)

// Fetcher handles PostgreSQL metadata retrieval
type Fetcher struct {
	connector *db.Connector
}

// NewFetcher creates a new metadata fetcher instance
func NewFetcher(dbURL string) (*Fetcher, error) {
	connector, err := db.New(dbURL)
	if err != nil {
		return nil, err
	}

	return &Fetcher{connector: connector}, nil
}

// Close closes the database connection
func (f *Fetcher) Close() error {
	return f.connector.Close()
}

// QueryObjects retrieves database objects matching the query options
func (f *Fetcher) QueryObjects(opts types.QueryOptions) ([]types.DBObject, error) {
	ctx := context.Background()
	return f.connector.QueryObjects(ctx, opts)
}

// SaveObjects exports database objects to files
// If continueOnError is true, it will log errors and continue; otherwise it will fail on first error
func (f *Fetcher) SaveObjects(objects []types.DBObject, outputDir string, continueOnError bool) error {
	log.Info("Exporting %d objects to %s (continueOnError: %v)", len(objects), outputDir, continueOnError)
	exporter := export.New(f.connector, outputDir)
	return exporter.ExportObjects(context.Background(), objects, continueOnError)
}

// GetAllSchemas returns a list of all schemas in the database
func (f *Fetcher) GetAllSchemas() ([]string, error) {
	ctx := context.Background()
	return f.connector.GetAllSchemas(ctx)
}

// Utility function to check if a type is valid
func IsValidType(t types.ObjectType) bool {
	return types.IsValidType(t)
}
