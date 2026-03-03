package google

import (
	"context"
	"fmt"
	"net/http"
	"os"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/calendar/v3"
	"google.golang.org/api/option"
)

type GoogleAdapter struct {
	id        string
	name      string
	client    *http.Client
	service   *calendar.Service
	config    *oauth2.Config
	credsFile string
	tokenFile string
	calendars map[string]string
}

func NewGoogleAdapter(id, name, credsFile, tokenFile string) *GoogleAdapter {
	return &GoogleAdapter{
		id:        id,
		name:      name,
		credsFile: credsFile,
		tokenFile: tokenFile,
		calendars: make(map[string]string),
	}
}

func (g *GoogleAdapter) ID() string   { return g.id }
func (g *GoogleAdapter) Name() string { return g.name }

// Login loads credentials and token, then initializes the Calendar service.
// Run `tsk auth` first to generate token.json.
func (g *GoogleAdapter) Login(ctx context.Context) error {
	b, err := os.ReadFile(g.credsFile)
	if err != nil {
		return fmt.Errorf("read credentials file: %w", err)
	}

	config, err := google.ConfigFromJSON(b, calendar.CalendarReadonlyScope)
	if err != nil {
		return fmt.Errorf("parse credentials: %w", err)
	}
	g.config = config

	tok, err := tokenFromFile(g.tokenFile)
	if err != nil {
		return fmt.Errorf("read token file (run tsk auth first): %w", err)
	}

	g.client = g.config.Client(ctx, tok)
	g.service, err = calendar.NewService(ctx, option.WithHTTPClient(g.client))
	if err != nil {
		return err
	}

	// Fetch calendar list to get names for all calendars
	if err := g.loadCalendarList(ctx); err != nil {
		return fmt.Errorf("load calendar list: %w", err)
	}

	return nil
}

// loadCalendarList fetches all calendars the user has access to.
func (g *GoogleAdapter) loadCalendarList(ctx context.Context) error {
	calList, err := g.service.CalendarList.List().Context(ctx).Do()
	if err != nil {
		return err
	}

	for _, cal := range calList.Items {
		g.calendars[cal.Id] = cal.Summary
	}
	return nil
}

// Calendars returns a list of available calendars (ID -> Name).
func (g *GoogleAdapter) Calendars() map[string]string {
	return g.calendars
}
