package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/dexidp/dex/connector"
	"github.com/dexidp/dex/storage"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/security"
)

type Authorizer struct {
	app     core.App
	storage storage.Storage
}

// user represent database User
type user struct {
	Email    string
	Disabled bool
	Groups   []string
}

func NewAuthorizer(app core.App, storage storage.Storage) *Authorizer {
	return &Authorizer{
		app:     app,
		storage: storage,
	}
}

// Authorize check for user authorization and may make updates to identity
func (a *Authorizer) Authorize(identity *connector.Identity, authID string) error {
	ctx := context.Background()
	authReq, err := a.storage.GetAuthRequest(ctx, authID)
	if err != nil {
		return err
	}

	// create user
	u, err := a.getUser(*identity)
	if err != nil {
		return err
	}

	if u.Disabled {
		return fmt.Errorf("user is disabled: %s", u.Email)
	}

	clientRecord, err := a.app.FindRecordById("clients", authReq.ClientID)
	if err != nil {
		return err
	}

	errs := a.app.ExpandRecord(clientRecord, []string{"allowedGroups"}, nil)
	if len(errs) > 0 {
		return fmt.Errorf("failed to expand client: %v", errs)
	}

	defer func() {
		// Update groups claim with user assigned groups
		for _, userGroup := range u.Groups {
			identity.Groups = append(identity.Groups, userGroup)
		}
	}()

	clientAllowedGroups := relations2Ids(clientRecord.ExpandedAll("allowedGroups"))
	if len(clientAllowedGroups) == 0 {
		return nil
	}

	isAllowed := false
	for _, allowGroup := range clientAllowedGroups {
		for _, userGroup := range u.Groups {
			if userGroup == allowGroup {
				isAllowed = true
				break
			}
		}

		if isAllowed {
			break
		}
	}

	if !isAllowed {
		return &connector.UserNotInRequiredGroupsError{
			UserID: identity.UserID,
			Groups: clientAllowedGroups,
		}
	}

	return nil
}

func (a *Authorizer) getUser(identity connector.Identity) (*user, error) {
	record, err := a.app.FindRecordById("users", identity.UserID)
	name := identity.Username
	if name == "" {
		name = identity.PreferredUsername
	}

	claims, _ := json.Marshal(identity)
	if err != nil && strings.Contains(err.Error(), "no row") {
		collection, _ := a.app.FindCollectionByNameOrId("users")
		record = core.NewRecord(collection)
		record.Set("id", identity.UserID)
		record.Set("name", name)
		record.Set("email", identity.Email)
		record.Set("verified", identity.EmailVerified)
		record.Set("claims", claims)
		record.Set("password", security.PseudorandomString(8))
		err = a.app.Save(record)
		if err != nil {
			return nil, err // Could be email duplicated
		}

		return &user{Email: identity.Email, Groups: []string{}, Disabled: false}, nil
	}

	if err != nil {
		return nil, err
	}

	if record.GetBool("disabled") {
		return &user{Email: identity.Email, Groups: []string{}, Disabled: true}, nil
	}

	record.Set("claims", claims)
	record.Set("name", name)
	if err = a.app.Save(record); err != nil {
		return nil, err
	}

	errs := a.app.ExpandRecord(record, []string{"groups"}, nil)
	if len(errs) > 0 {
		return nil, fmt.Errorf("failed to expand user: %v", errs)
	}

	return &user{
		Email:    record.GetString("email"),
		Groups:   relations2Ids(record.ExpandedAll("groups")),
		Disabled: record.GetBool("disabled"),
	}, nil
}

func relations2Ids(records []*core.Record) []string {
	ids := make([]string, len(records))
	for i, r := range records {
		ids[i] = r.Id
	}

	return ids
}
