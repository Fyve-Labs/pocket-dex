package storage

import (
	"context"
	"fmt"

	"github.com/dexidp/dex/storage"
	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
)

func toStorageClient(r *core.Record) storage.Client {
	return storage.Client{
		ID:                r.Id,
		Name:              r.GetString("name"),
		Secret:            r.GetString("secret"),
		Public:            r.GetBool("public"),
		LogoURL:           r.GetString("logoURL"),
		RedirectURIs:      r.GetStringSlice("redirectURIs"),
		AllowedConnectors: records2Ids(r.ExpandedAll("allowedConnectors")),
		TrustedPeers:      records2Ids(r.ExpandedAll("trustedPeers")),
	}
}

func records2Ids(records []*core.Record) []string {
	ids := make([]string, len(records))
	for i, r := range records {
		ids[i] = r.Id
	}

	return ids
}

func (s pbStorage) GetClient(ctx context.Context, id string) (storage.Client, error) {
	record, err := s.App.FindRecordById("clients", id)
	if err != nil {
		return storage.Client{}, convertDBError("get client: %w", err)
	}

	return toStorageClient(record), nil
}

func (s pbStorage) ListClients(ctx context.Context) ([]storage.Client, error) {
	records, err := s.App.FindAllRecords("clients", dbx.HashExp{"disabled": false})
	if err != nil {
		return []storage.Client{}, convertDBError("get clients: %w", err)
	}

	clients := make([]storage.Client, len(records))
	for i, record := range records {
		errs := s.App.ExpandRecord(record, []string{"allowedConnectors", "trustedPeers"}, nil)
		if len(errs) > 0 {
			return []storage.Client{}, fmt.Errorf("failed to expand: %v", errs)
		}
		clients[i] = toStorageClient(record)
	}

	return clients, nil
}
