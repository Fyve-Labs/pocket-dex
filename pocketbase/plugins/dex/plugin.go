package dex

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"github.com/Fyve-Labs/pocket-dex/internal/pb_admin"
	"github.com/Fyve-Labs/pocket-dex/internal/service"
	"github.com/Fyve-Labs/pocket-dex/pocketbase/plugins/dex/oidc2"
	"github.com/Fyve-Labs/pocket-dex/storage"
	dex "github.com/dexidp/dex/server"
	"github.com/dexidp/dex/server/signer"
	"github.com/gosimple/slug"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/security"
)

type plugin struct {
	app core.App
}

func MustRegister(app core.App) {
	p := &plugin{app: app}

	p.app.OnServe().BindFunc(p.setupDexServer)
	p.app.OnRecordCreateRequest("clients", "connectors").BindFunc(p.autoGenerateIdHook)
	p.app.OnRecordCreateRequest("clients").BindFunc(p.autoGenerateSecretHook)
}

func (p *plugin) setupDexServer(e *core.ServeEvent) error {
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
		AlwaysShowLoginScreen:      false,
		ContinueOnConnectorFailure: true,
		SkipApprovalScreen:         true,
		Issuer:                     getEnv("DEX_ISSUER", "http://127.0.0.1:8090"),
		Storage:                    s,
		Logger:                     logger,
		Now:                        now,
		Signer:                     signerInstance,
		IDTokensValidFor:           idTokensValidFor,
	}

	authorizer := service.NewAuthorizer(e.App, s)
	oidc2Config := &oidc2.Config{Authorizer: authorizer}
	dex.ConnectorsConfig["oidc2"] = func() dex.ConnectorConfig { return oidc2Config }
	dexServer, err := dex.NewServer(context.Background(), serverConfig)
	if err != nil {
		return err
	}

	e.Router.Any("/{path...}", func(e *core.RequestEvent) error {
		dexServer.ServeHTTP(e.Response, e.Request)
		return nil
	})

	go func() {
		// Start PocketBase Admin in Tailscale network
		pb_admin.StartServer(e.App)
	}()

	return e.Next()
}

func (p *plugin) autoGenerateIdHook(e *core.RecordRequestEvent) error {
	record := e.Record
	rawID, _ := record.GetRaw("id").(string)
	if rawID == "" {
		record.SetRaw("id", slug.Make(record.GetString("name")))
	}

	return e.Next()
}

func (p *plugin) autoGenerateSecretHook(e *core.RecordRequestEvent) error {
	record := e.Record
	rawSecret, _ := record.GetRaw("secret").(string)
	public := record.GetBool("public")
	if rawSecret == "" && !public {
		record.SetRaw("secret", security.RandomStringWithAlphabet(16, "abcdefghijklmnopqrstuvwxyz0123456789"))
	}

	return e.Next()
}

func getEnv(name, defaultValue string) string {
	if val, ok := os.LookupEnv(name); ok {
		return val
	}

	return defaultValue
}
