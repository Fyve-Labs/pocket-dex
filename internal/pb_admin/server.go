package pb_admin

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/ui"
	"tailscale.com/tsnet"
)

var hs *pbAdminServer

func StartServer(app core.App) {
	if hs != nil {
		panic("StartServer called twice")
	}

	hs = &pbAdminServer{app: app}
	app.OnTerminate().BindFunc(func(e *core.TerminateEvent) error {
		if err := hs.Shutdown(); err != nil {
			return err
		}

		return e.Next()
	})

	if err := hs.Start(); err != nil {
		panic(err)
	}
}

type pbAdminServer struct {
	app    core.App
	server *http.Server
}

func (hs *pbAdminServer) Start() error {
	pbRouter, err := apis.NewRouter(hs.app)
	if err != nil {
		return err
	}

	pbRouter.GET("/_/{path...}", apis.Static(ui.DistDirFS, false)).
		BindFunc(func(e *core.RequestEvent) error {
			if e.Request.PathValue(apis.StaticWildcardParam) != "" {
				e.Response.Header().Set("Cache-Control", "max-age=1209600, stale-while-revalidate=86400")
			}

			// add a default CSP
			if e.Response.Header().Get("Content-Security-Policy") == "" {
				e.Response.Header().Set("Content-Security-Policy", "default-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' http://127.0.0.1:* https://tile.openstreetmap.org data: blob:; connect-src 'self' http://127.0.0.1:* https://nominatim.openstreetmap.org; script-src 'self' 'sha256-GRUzBA7PzKYug7pqxv5rJaec5bwDCw1Vo6/IXwvD3Tc='")
			}

			return e.Next()
		}).
		Bind(apis.Gzip())

	mux, err := pbRouter.BuildMux()
	if err != nil {
		return err
	}

	hostname := "pocket-dex"
	if v := os.Getenv("TS_HOSTNAME"); v != "" {
		hostname = v
	}

	s := &tsnet.Server{
		Dir:        filepath.Join(hs.app.DataDir(), "tsnet"),
		Hostname:   hostname,
		ControlURL: os.Getenv("TS_SERVER"),
	}

	if _, err := s.Up(context.Background()); err != nil {
		return fmt.Errorf("tailscale did not come up: %w", err)
	}

	l80, err := s.Listen("tcp", ":80")
	if err != nil {
		return fmt.Errorf("creating HTTP listener: %v", err)
	}

	hs.server = &http.Server{Handler: mux}
	if err := hs.server.Serve(l80); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("serving HTTPS: %v", err)
	}

	return nil
}

func (hs *pbAdminServer) Shutdown() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return hs.server.Shutdown(ctx)
}
