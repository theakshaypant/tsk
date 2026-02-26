package outlook

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	msgraphsdk "github.com/microsoftgraph/msgraph-sdk-go"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/microsoft"
)

// tokenCredential bridges our saved OAuth2 token into the Azure SDK's
// TokenCredential interface, allowing the Microsoft Graph SDK to
// authenticate requests.
type tokenCredential struct {
	adapter *OutlookAdapter
}

func (c *tokenCredential) GetToken(ctx context.Context, opts policy.TokenRequestOptions) (azcore.AccessToken, error) {
	tok, err := c.adapter.accessToken(ctx)
	if err != nil {
		return azcore.AccessToken{}, err
	}
	c.adapter.tokenMu.Lock()
	expiry := c.adapter.token.Expiry
	c.adapter.tokenMu.Unlock()
	return azcore.AccessToken{
		Token:     tok,
		ExpiresOn: expiry,
	}, nil
}

// OutlookAdapter implements the calendar provider for Microsoft Outlook / Office 365
// using the official Microsoft Graph SDK.
type OutlookAdapter struct {
	id        string
	name      string
	clientID  string
	tenantID  string
	tokenFile string
	calendars map[string]string

	token   *oauth2.Token
	tokenMu sync.Mutex
	client  *msgraphsdk.GraphServiceClient
}

func NewOutlookAdapter(id, name, clientID, tenantID, tokenFile string) *OutlookAdapter {
	if tenantID == "" {
		tenantID = "common"
	}
	return &OutlookAdapter{
		id:        id,
		name:      name,
		clientID:  clientID,
		tenantID:  tenantID,
		tokenFile: tokenFile,
		calendars: make(map[string]string),
	}
}

func (o *OutlookAdapter) ID() string   { return o.id }
func (o *OutlookAdapter) Name() string { return o.name }

// OAuthConfig returns the OAuth2 configuration for Microsoft identity platform.
// Used by the auth command to run the initial OAuth flow.
func (o *OutlookAdapter) OAuthConfig() *oauth2.Config {
	return &oauth2.Config{
		ClientID:    o.clientID,
		Endpoint:    microsoft.AzureADEndpoint(o.tenantID),
		RedirectURL: "http://localhost:8085/callback",
		Scopes: []string{
			"https://graph.microsoft.com/Calendars.Read",
			"https://graph.microsoft.com/User.Read",
			"offline_access",
		},
	}
}

// Login loads the saved OAuth token and initializes the Graph SDK client.
func (o *OutlookAdapter) Login(ctx context.Context) error {
	tok, err := tokenFromFile(o.tokenFile)
	if err != nil {
		return fmt.Errorf("read token file (run 'tsk auth' first): %w", err)
	}

	if tok.AccessToken == "" {
		return fmt.Errorf("token file has no access token — delete %s and run 'tsk auth' again", o.tokenFile)
	}

	o.token = tok

	cred := &tokenCredential{adapter: o}
	client, err := msgraphsdk.NewGraphServiceClientWithCredentials(cred, []string{
		"https://graph.microsoft.com/.default",
	})
	if err != nil {
		return fmt.Errorf("create graph client: %w", err)
	}
	o.client = client

	if err := o.loadCalendarList(ctx); err != nil {
		return fmt.Errorf("load calendar list: %w", err)
	}

	return nil
}

// accessToken returns a valid access token, refreshing if expired.
func (o *OutlookAdapter) accessToken(ctx context.Context) (string, error) {
	o.tokenMu.Lock()
	defer o.tokenMu.Unlock()

	if o.token.Valid() {
		return o.token.AccessToken, nil
	}

	// Token expired — refresh it
	src := o.OAuthConfig().TokenSource(ctx, o.token)
	newTok, err := src.Token()
	if err != nil {
		return "", fmt.Errorf("token expired and refresh failed (delete %s and run 'tsk auth'): %w", o.tokenFile, err)
	}

	o.token = newTok

	// Persist the refreshed token
	if f, err := os.Create(o.tokenFile); err == nil {
		json.NewEncoder(f).Encode(newTok)
		f.Close()
	}

	return newTok.AccessToken, nil
}

// Calendars returns all available calendars (ID → Name).
func (o *OutlookAdapter) Calendars() map[string]string {
	return o.calendars
}

// loadCalendarList fetches all calendars the user has access to.
func (o *OutlookAdapter) loadCalendarList(ctx context.Context) error {
	result, err := o.client.Me().Calendars().Get(ctx, nil)
	if err == nil {
		for _, cal := range result.GetValue() {
			id := cal.GetId()
			name := cal.GetName()
			if id != nil && name != nil {
				o.calendars[*id] = *name
			}
		}
	}

	return nil
}
