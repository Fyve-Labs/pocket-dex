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

	//_ "github.com/Fyve-Labs/dex-pocket/migrations"
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
		s := storage.New(e.App)

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

		serverConfig.Storage = s

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

	app.OnRecordCreate("clients", "connectors").Bind(&hook.Handler[*core.RecordEvent]{
		Func: func(e *core.RecordEvent) error {
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
						record.SetRaw("secret", security.RandomString(32))
					}
				}
			}
			return e.Next()
		},
		Priority: -100, // Need to fire before __pbRecordSystemHook__
	})

	err := app.Start()
	if err != nil {
		log.Fatal(err)
	}
}
