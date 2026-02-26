package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"golang.org/x/oauth2/microsoft"
	"google.golang.org/api/calendar/v3"
)

const (
	redirectPort = "8085"
	redirectURL  = "http://localhost:" + redirectPort + "/callback"
)

var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "Authenticate with your calendar provider",
	Long: `Authenticate with your calendar provider using OAuth.

For Google Calendar:
  1. Starts a local server to receive the OAuth callback
  2. Opens your browser to sign in with Google
  3. Saves the token for future use

For Outlook / Office 365:
  1. Starts a local server to receive the OAuth callback
  2. Opens your browser to sign in with Microsoft
  3. Saves the token for future use

The provider is determined by your profile configuration (provider: google|outlook).`,
	RunE:              runAuth,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error { return nil }, // Skip adapter init
}

func init() {
	rootCmd.AddCommand(authCmd)
}

func runAuth(cmd *cobra.Command, args []string) error {
	provider := viper.GetString("provider")

	switch provider {
	case "google":
		return runGoogleAuth(cmd, args)
	case "outlook":
		return runOutlookAuth(cmd, args)
	default:
		return fmt.Errorf("unknown provider: %s (supported: google, outlook)", provider)
	}
}

func runGoogleAuth(_ *cobra.Command, _ []string) error {
	credsFile := expandPath(viper.GetString("credentials_file"))
	tokenFile := expandPath(viper.GetString("token_file"))

	b, err := os.ReadFile(credsFile)
	if err != nil {
		return fmt.Errorf("unable to read credentials file: %w\n\nSetup guide: https://github.com/theakshaypant/tsk/tree/main/docs/google_setup.md", err)
	}

	config, err := google.ConfigFromJSON(b, calendar.CalendarReadonlyScope)
	if err != nil {
		return fmt.Errorf("unable to parse credentials: %w", err)
	}

	config.RedirectURL = redirectURL

	tok, err := getTokenViaLocalServer(config, "Google", oauth2.AccessTypeOffline, oauth2.ApprovalForce)
	if err != nil {
		return fmt.Errorf("failed to get token: %w", err)
	}

	if err := saveToken(tokenFile, tok); err != nil {
		return fmt.Errorf("failed to save token: %w", err)
	}

	fmt.Println("\n✅ Authentication successful!")
	fmt.Printf("📁 Token saved to %s\n", tokenFile)
	fmt.Println("\nYou can now run 'tsk' to see your Google Calendar events.")

	return nil
}

func runOutlookAuth(_ *cobra.Command, _ []string) error {
	clientID := viper.GetString("client_id")
	if clientID == "" {
		return fmt.Errorf("client_id not configured\n\nAdd it to your profile config:\n  client_id: \"your-azure-app-client-id\"\n\nSetup guide: https://github.com/theakshaypant/tsk/tree/main/docs/outlook_setup.md")
	}

	tenantID := viper.GetString("tenant_id")
	if tenantID == "" {
		tenantID = "common"
	}

	tokenFile := expandPath(viper.GetString("token_file"))

	config := &oauth2.Config{
		ClientID:    clientID,
		Endpoint:    microsoft.AzureADEndpoint(tenantID),
		RedirectURL: redirectURL,
		Scopes: []string{
			"https://graph.microsoft.com/Calendars.Read",
			"https://graph.microsoft.com/User.Read",
			"offline_access",
		},
	}

	tok, err := getTokenViaLocalServer(config, "Microsoft", oauth2.SetAuthURLParam("prompt", "consent"))
	if err != nil {
		return fmt.Errorf("failed to get token: %w", err)
	}

	if err := saveToken(tokenFile, tok); err != nil {
		return fmt.Errorf("failed to save token: %w", err)
	}

	fmt.Println("\n✅ Authentication successful!")
	fmt.Printf("📁 Token saved to %s\n", tokenFile)
	fmt.Println("\nYou can now run 'tsk' to see your Outlook calendar events.")

	return nil
}

func getTokenViaLocalServer(config *oauth2.Config, providerName string, authOpts ...oauth2.AuthCodeOption) (*oauth2.Token, error) {
	codeChan := make(chan string, 1)
	errChan := make(chan error, 1)

	server := &http.Server{Addr: ":" + redirectPort}
	mux := http.NewServeMux()

	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		code := r.URL.Query().Get("code")
		if code == "" {
			errMsg := r.URL.Query().Get("error")
			http.Error(w, "Authorization failed: "+errMsg, http.StatusBadRequest)
			errChan <- fmt.Errorf("authorization failed: %s", errMsg)
			return
		}

		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, `
			<!DOCTYPE html>
			<html>
			<head>
				<title>Authorization Successful</title>
				<style>
					body { font-family: -apple-system, sans-serif; display: flex; 
					       justify-content: center; align-items: center; height: 100vh;
					       margin: 0; background: #1a1a1a; color: #fff; }
					.card { background: #2d2d2d; padding: 40px; border-radius: 12px; 
					        box-shadow: 0 2px 10px rgba(0,0,0,0.3); text-align: center; }
					h1 { color: #4ade80; margin-bottom: 10px; }
					p { color: #a1a1aa; }
				</style>
			</head>
			<body>
				<div class="card">
					<h1>Authorization Successful</h1>
					<p>You can close this window and return to the terminal.</p>
				</div>
			</body>
			</html>
		`)

		codeChan <- code
	})

	server.Handler = mux

	go func() {
		if err := server.ListenAndServe(); err != http.ErrServerClosed {
			errChan <- err
		}
	}()

	authURL := config.AuthCodeURL("state-token", authOpts...)

	fmt.Printf("🔐 Opening browser for %s authorization...\n", providerName)
	fmt.Println()

	if err := openBrowser(authURL); err != nil {
		fmt.Println("⚠️  Couldn't open browser automatically.")
		fmt.Println("   Please open this URL manually:")
		fmt.Println(authURL)
	}

	fmt.Println("⏳ Waiting for authorization...")

	var code string
	select {
	case code = <-codeChan:
	case err := <-errChan:
		server.Shutdown(context.Background())
		return nil, err
	case <-time.After(5 * time.Minute):
		server.Shutdown(context.Background())
		return nil, fmt.Errorf("timeout waiting for authorization")
	}

	server.Shutdown(context.Background())

	tok, err := config.Exchange(context.Background(), code)
	if err != nil {
		return nil, fmt.Errorf("failed to exchange code: %w", err)
	}

	return tok, nil
}

func openBrowser(url string) error {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		return fmt.Errorf("unsupported platform")
	}

	return cmd.Start()
}

func saveToken(path string, token *oauth2.Token) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return json.NewEncoder(f).Encode(token)
}
