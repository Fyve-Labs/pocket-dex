package storage

import (
	"context"
	"fmt"

	"github.com/dexidp/dex/storage"
	"github.com/dexidp/dex/storage/memory"
	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
)

const (
	ConnectorCollectionName = "connectors"
)

type pbStorage struct {
	storage.Storage

	App core.App
}

func New(app core.App) storage.Storage {
	s := memory.New(app.Logger())
	s = WithPocketbaseStorage(s, app)

	return s
}

func WithPocketbaseStorage(s storage.Storage, app core.App) storage.Storage {
	return pbStorage{s, app}
}

func toStorageConnector(r *core.Record) storage.Connector {
	return storage.Connector{
		ID:         r.Id,
		Type:       r.GetString("type"),
		Name:       r.GetString("name"),
		Config:     []byte(r.GetString("config")),
		GrantTypes: r.GetStringSlice("grantTypes"),
	}
}

func (s pbStorage) GetConnector(ctx context.Context, id string) (storage.Connector, error) {
	record, err := s.App.FindRecordById(ConnectorCollectionName, id)
	if err != nil {
		return storage.Connector{}, convertDBError("get connector: %w", err)
	}

	return toStorageConnector(record), nil
}

func (s pbStorage) ListConnectors(ctx context.Context) ([]storage.Connector, error) {
	records, err := s.App.FindAllRecords(ConnectorCollectionName, dbx.HashExp{"disabled": false})
	if err != nil {
		return []storage.Connector{}, convertDBError("get connectors: %w", err)
	}

	if len(records) == 0 {
		return s.Storage.ListConnectors(ctx)
	}

	connectors := make([]storage.Connector, len(records))
	for i, record := range records {
		connectors[i] = toStorageConnector(record)
	}

	return connectors, nil
}

func convertDBError(t string, err error) error {
	return fmt.Errorf(t, err)
}
