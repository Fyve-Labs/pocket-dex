package oidc2

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/Fyve-Labs/pocket-dex/internal/service"
	"github.com/dexidp/dex/connector"
	"github.com/dexidp/dex/connector/oidc"
)

type Config struct {
	oidc.Config

	Authorizer *service.Authorizer
}

func (c *Config) Open(id string, logger *slog.Logger) (connector.Connector, error) {
	conn, err := c.Config.Open(id, logger)
	if err != nil {
		return nil, err
	}

	return &oidc2Connector{conn: conn, authorizer: c.Authorizer}, nil
}

var (
	_ connector.CallbackConnector      = (*oidc2Connector)(nil)
	_ connector.RefreshConnector       = (*oidc2Connector)(nil)
	_ connector.TokenIdentityConnector = (*oidc2Connector)(nil)
)

type oidc2Connector struct {
	conn       connector.Connector
	authorizer *service.Authorizer
}

func (c *oidc2Connector) LoginURL(s connector.Scopes, callbackURL, state string) (string, []byte, error) {
	redirectURL, connData, err := c.conn.(connector.CallbackConnector).LoginURL(s, callbackURL, state)
	if err != nil {
		return redirectURL, connData, err
	}
	var connDater map[string]interface{}
	err = json.Unmarshal(connData, &connDater)
	if err != nil {
		return redirectURL, connData, err
	}

	if _, ok := connDater["state"]; !ok {
		// Attach state to connector data for retrieval later
		connDater["state"] = state
	}

	newConnData, err := json.Marshal(connDater)
	if err != nil {
		return redirectURL, connData, err
	}

	return redirectURL, newConnData, nil
}

func (c *oidc2Connector) HandleCallback(s connector.Scopes, connData []byte, r *http.Request) (connector.Identity, error) {
	identity, err := c.conn.(connector.CallbackConnector).HandleCallback(s, connData, r)
	if err != nil {
		return connector.Identity{}, err
	}

	var connDater map[string]string
	err = json.Unmarshal(connData, &connDater)
	if err != nil {
		return identity, err
	}

	if _, ok := connDater["state"]; !ok {
		return identity, err
	}

	authID := connDater["state"]
	err = c.authorizer.Authorize(&identity, authID)
	if err != nil {
		return identity, fmt.Errorf("authorize auth req: %w", err)
	}

	return identity, nil
}

func (c *oidc2Connector) Refresh(ctx context.Context, s connector.Scopes, identity connector.Identity) (connector.Identity, error) {
	return c.conn.(connector.RefreshConnector).Refresh(ctx, s, identity)
}

func (c *oidc2Connector) TokenIdentity(ctx context.Context, subjectTokenType, subjectToken string) (connector.Identity, error) {
	return c.conn.(connector.TokenIdentityConnector).TokenIdentity(ctx, subjectTokenType, subjectToken)
}
