package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

func init() {
	m.Register(func(app core.App) error {
		collection, err := app.FindCollectionByNameOrId("connectors")
		if err != nil {
			return err
		}

		record := core.NewRecord(collection)
		record.Set("id", "local")
		record.Set("type", "local")
		record.Set("name", "Email")
		return app.SaveNoValidate(record)
	}, func(app core.App) error {
		return nil
	})
}
