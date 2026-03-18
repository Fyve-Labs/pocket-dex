package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"time"

	dex "github.com/dexidp/dex/server"
	"github.com/dexidp/dex/server/signer"
	"github.com/gosimple/slug"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/plugins/ghupdate"
	"github.com/pocketbase/pocketbase/plugins/migratecmd"
	"github.com/pocketbase/pocketbase/tools/hook"
	"github.com/pocketbase/pocketbase/tools/security"
	"github.com/spf13/cobra"

	_ "github.com/Fyve-Labs/pocket-dex/migrations"
	"github.com/Fyve-Labs/pocket-dex/storage"
)

func main() {
	app := pocketbase.NewWithConfig(pocketbase.Config{
		DefaultDataDir: getEnv("DATA_PATH", "./pb_data"),
	})

	migratecmd.MustRegister(app, app.RootCmd, migratecmd.Config{
		Dir: "./migrations",
	})

	// GitHub selfupdate
	ghupdate.MustRegister(app, app.RootCmd, ghupdate.Config{
		Owner: "Fyve-Labs",
		Repo:  "pocket-dex",
	})

	app.RootCmd.AddCommand(commandVersion())

	app.OnServe().BindFunc(func(e *core.ServeEvent) error {
		now := func() time.Time { return time.Now().UTC() }
		idTokensValidFor := 24 * time.Hour
		keysRotationPeriod := getEnv("DEX_EXPIRY_SIGNING_KEYS", "6h")

		logger := e.App.Logger()
		s, err := storage.New(e.App, getEnv("DEX_STORAGE_SQLITE3_CONFIG_FILE", filepath.Join(e.App.DataDir(), "dex.db")))
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
			Issuer:                     getEnv("DEX_ISSUER", "http://127.0.0.1:8090"),
			Storage:                    s,
			Logger:                     app.Logger(),
			Now:                        now,
			ContinueOnConnectorFailure: false,
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

var version = "DEV"

func commandVersion() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the version and exit",
		Run: func(_ *cobra.Command, _ []string) {
			fmt.Printf(
				"Pocket Dex Version: %s\nGo Version: %s\nGo OS/ARCH: %s %s\n",
				version,
				runtime.Version(),
				runtime.GOOS,
				runtime.GOARCH,
			)
		},
	}
}

func getEnv(name, defaultValue string) string {
	if val, ok := os.LookupEnv(name); ok {
		return val
	}

	return defaultValue
}
