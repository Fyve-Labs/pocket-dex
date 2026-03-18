package main

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/Fyve-Labs/pocket-dex/storage"
	"github.com/dexidp/dex/pkg/featureflags"
	"github.com/dexidp/dex/server/signer"
	"github.com/gosimple/slug"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/plugins/migratecmd"
	"github.com/pocketbase/pocketbase/tools/hook"
	"github.com/pocketbase/pocketbase/tools/security"

	_ "github.com/Fyve-Labs/pocket-dex/migrations"
	dex "github.com/dexidp/dex/server"
)

func main() {
	isDev := os.Getenv("ENV") == "dev"
	app := pocketbase.NewWithConfig(pocketbase.Config{
		DefaultDev: isDev,
	})

	// enable auto-creation of migration files when making collection changes in the Admin UI
	migratecmd.MustRegister(app, app.RootCmd, migratecmd.Config{
		Automigrate: isDev,
		Dir:         "./migrations",
	})

	app.OnServe().BindFunc(func(e *core.ServeEvent) error {
		now := func() time.Time { return time.Now().UTC() }
		idTokensValidFor := 24 * time.Hour
		keysRotationPeriod := "6h"

		logger := e.App.Logger()
		s, err := storage.New(e.App, "./pb_data/dex.db")
		if err != nil {
			return err
		}

		localConfig := signer.LocalConfig{KeysRotationPeriod: keysRotationPeriod}
		signerInstance, err := localConfig.Open(context.Background(), s, idTokensValidFor, now, logger)
		if err != nil {
			return err
		}

		serverConfig := dex.Config{
			SkipApprovalScreen:         true,
			AlwaysShowLoginScreen:      false,
			Issuer:                     "http://127.0.0.1:8090",
			Storage:                    s,
			Logger:                     app.Logger(),
			Now:                        now,
			ContinueOnConnectorFailure: featureflags.ContinueOnConnectorFailure.Enabled(),
			Signer:                     signerInstance,
			IDTokensValidFor:           idTokensValidFor,
		}
		dexServer, err := dex.NewServer(context.Background(), serverConfig)
		if err != nil {
			return err
		}

		e.Router.Any("/{path...}", func(e *core.RequestEvent) error {
			dexServer.ServeHTTP(e.Response, e.Request)
			return nil
		})

		return e.Next()
	})

	app.OnRecordCreateRequest("clients", "connectors").Bind(&hook.Handler[*core.RecordRequestEvent]{
		Func: func(e *core.RecordRequestEvent) error {
			record := e.Record
			if record.IsNew() {
				rawID, _ := record.GetRaw("id").(string)
				if rawID == "" {
					record.SetRaw("id", slug.Make(record.GetString("name")))
				}

				// Auto generate Client secret
				if record.Collection().Name == "clients" {
					rawSecret, _ := record.GetRaw("secret").(string)
					public := record.GetBool("public")
					if rawSecret == "" && !public {
						record.SetRaw("secret", security.RandomStringWithAlphabet(16, "abcdefghijklmnopqrstuvwxyz0123456789"))
					}
				}
			}
			return e.Next()
		},
	})

	err := app.Start()
	if err != nil {
		log.Fatal(err)
	}
}
