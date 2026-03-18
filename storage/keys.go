package storage

import (
	"context"
	"encoding/json"

	"github.com/dexidp/dex/storage"
	"github.com/go-jose/go-jose/v4"
	"github.com/pocketbase/pocketbase/core"
)

const keysRowID = "keys"

func toStorageKeys(r *core.Record) storage.Keys {
	var signingKey jose.JSONWebKey
	var signingKeyPub jose.JSONWebKey
	var verificationKeys []storage.VerificationKey

	_ = r.UnmarshalJSONField("signing_key", &signingKey)
	_ = r.UnmarshalJSONField("signing_key_pub", &signingKeyPub)
	_ = r.UnmarshalJSONField("verification_keys", &verificationKeys)

	return storage.Keys{
		SigningKey:       &signingKey,
		SigningKeyPub:    &signingKeyPub,
		VerificationKeys: verificationKeys,
		NextRotation:     r.GetDateTime("next_rotation").Time(),
	}
}

func getKeys(ctx context.Context, app core.App) (storage.Keys, error) {
	record, err := app.FindRecordById("keys", keysRowID)
	if err != nil {
		return storage.Keys{}, storage.ErrNotFound
	}

	return toStorageKeys(record), nil
}

func (s pbStorage) GetKeys(ctx context.Context) (storage.Keys, error) {
	return getKeys(ctx, s.App)
}

func (s pbStorage) UpdateKeys(ctx context.Context, updater func(old storage.Keys) (storage.Keys, error)) error {
	return s.App.RunInTransaction(func(txApp core.App) error {
		collection, err := txApp.FindCollectionByNameOrId("keys")
		if err != nil {
			return err
		}
		firstUpdate := false
		storageKeys, err := getKeys(ctx, txApp)
		if err != nil {
			firstUpdate = true
		}

		newKeys, err := updater(storageKeys)
		if err != nil {
			return err
		}

		updateRecord := func(record *core.Record) error {
			verificationKeys, err := json.Marshal(newKeys.VerificationKeys)
			if err != nil {
				return err
			}

			signingKey, err := json.Marshal(newKeys.SigningKey)
			if err != nil {
				return err
			}

			signingKeyPub, err := json.Marshal(newKeys.SigningKeyPub)
			if err != nil {
				return err
			}

			record.Set("verification_keys", verificationKeys)
			record.Set("signing_key", signingKey)
			record.Set("signing_key_pub", signingKeyPub)
			record.Set("next_rotation", newKeys.NextRotation.UTC())

			return nil
		}

		if firstUpdate {
			record := core.NewRecord(collection)
			record.Set("id", keysRowID)

			if err = updateRecord(record); err != nil {
				return err
			}
			return txApp.Save(record)

		}

		record, err := txApp.FindRecordById("keys", keysRowID)
		if err != nil {
			return err
		}

		if err = updateRecord(record); err != nil {
			return err
		}

		return txApp.Save(record)
	})
}
