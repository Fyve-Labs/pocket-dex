package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"runtime"

	"github.com/Fyve-Labs/pocket-dex/internal/pb_admin"
	_ "github.com/Fyve-Labs/pocket-dex/migrations"
	"github.com/Fyve-Labs/pocket-dex/pocketbase/plugins/dex"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/plugins/ghupdate"
	"github.com/pocketbase/pocketbase/plugins/migratecmd"
	"github.com/pocketbase/pocketbase/tools/hook"
	"github.com/pocketbase/pocketbase/tools/router"
	"github.com/spf13/cobra"
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

	app.OnServe().Bind(&hook.Handler[*core.ServeEvent]{
		Func: func(e *core.ServeEvent) error {
			go func() {
				// Start PocketBase Admin in Tailscale network
				pb_admin.StartServer(e.App)
			}()

			return e.Next()
		},
		Priority: 9999,
	})

	// Reset Pocketbase original Router
	app.OnServe().Bind(&hook.Handler[*core.ServeEvent]{
		Func: func(e *core.ServeEvent) error {
			pbRouter := router.NewRouter(func(w http.ResponseWriter, r *http.Request) (*core.RequestEvent, router.EventCleanupFunc) {
				event := new(core.RequestEvent)
				event.Response = w
				event.Request = r
				event.App = app

				return event, nil
			})
			pbRouter.Bind(apis.BodyLimit(apis.DefaultMaxBodySize))
			e.Router = pbRouter

			return e.Next()
		},
		Priority: -9999,
	})

	dex.MustRegister(app)

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
