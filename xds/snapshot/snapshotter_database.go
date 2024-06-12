package snapshot

import (
	"context"

	"github.com/edgedb/edgedb-go"
)

// DatabaseProvider is an interface for database providers.
type DatabaseProvider interface {
	GetDatabase(ctx context.Context) (Database, error)
}

// Database is an interface for database operations.
type Database interface {
	// Add database-specific methods as needed
}

// EdgeDBProvider is a provider for EdgeDB.
type EdgeDBProvider struct {
	client *edgedb.Client
}

// NewEdgeDBProvider creates a new EdgeDBProvider.
func NewEdgeDBProvider(client *edgedb.Client) *EdgeDBProvider {
	return &EdgeDBProvider{client: client}
}

// GetDatabase returns the EdgeDB database.
func (p *EdgeDBProvider) GetDatabase(ctx context.Context) (Database, error) {
	return p.client, nil
}
