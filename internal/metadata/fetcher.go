package metadata

import (
	"context"

	"github.com/shkamensky/pgmeta/internal/log"
	"github.com/shkamensky/pgmeta/internal/metadata/db"
	"github.com/shkamensky/pgmeta/internal/metadata/export"
	"github.com/shkamensky/pgmeta/internal/metadata/types"
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
func (f *Fetcher) SaveObjects(objects []types.DBObject, outputDir string) error {
	log.Info("Exporting %d objects to %s", len(objects), outputDir)
	exporter := export.New(f.connector, outputDir)
	return exporter.ExportObjects(context.Background(), objects)
}

// Utility function to check if a type is valid
func IsValidType(t types.ObjectType) bool {
	return types.IsValidType(t)
}