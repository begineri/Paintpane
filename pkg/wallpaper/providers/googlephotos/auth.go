package googlephotos

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/dixieflatline76/Spice/v2/pkg/wallpaper"
	"github.com/dixieflatline76/Spice/v2/util/log"
)

// Authenticator handles the OAuth flow for Google Photos.
type Authenticator struct {
	cfg        *wallpaper.Config
	httpClient *http.Client
}

// NewAuthenticator creates a new Google Photos Authenticator.
func NewAuthenticator(cfg *wallpaper.Config, client *http.Client) *Authenticator {
	return &Authenticator{
		cfg:        cfg,
		httpClient: client,
	}
}

// StartOAuthFlow initiates the OAuth flow using PKCE.
func (a *Authenticator) StartOAuthFlow(openURLFunc func(*url.URL) error) error {
	// 1. Generate PKCE Verifier and Challenge
	verifier, err := generateRandomString(32)
	if err != nil {
		return fmt.Errorf("failed to generate verifier: %w", err)
	}
	challenge := generateCodeChallenge(verifier)

	state, err := generateRandomString(32)
	if err != nil {
		return fmt.Errorf("failed to generate state: %w", err)
	}

	// 2. Setup Local Server for Callback
	codeChan := make(chan string)
	errChan := make(chan error)

	mux := http.NewServeMux()
	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("state") != state {
			http.Error(w, "Invalid state", http.StatusBadRequest)
			errChan <- fmt.Errorf("invalid state parameter")
			return
		}

		code := r.URL.Query().Get("code")
		if code == "" {
			http.Error(w, "Missing code", http.StatusBadRequest)
			errChan <- fmt.Errorf("missing code parameter")
			return
		}

		w.Header().Set("Content-Type", "text/html")
		if _, err := w.Write([]byte(`
			<html>
			<head>
				<meta charset="UTF-8">
				<title>Paintpane: Connected</title>
				<style>
					body {
						background: #f7f5f0;
						font-family: "Segoe UI", system-ui, sans-serif;
						color: #1d1b18;
						text-align: center;
						display: flex;
						flex-direction: column;
						align-items: center;
						justify-content: center;
						height: 100vh;
						margin: 0;
					}
					.box {
						background: #fff;
						border: 1px solid #e6e1d8;
						border-radius: 12px;
						padding: 32px 40px;
					}
					h1 { font-size: 22px; margin: 0 0 12px; }
					p { margin: 0; color: #6b665e; line-height: 1.6; }
				</style>
			</head>
			<body>
				<div class="box">
					<h1>Connected to Google Photos</h1>
					<p>You can close this tab and return to Paintpane.</p>
				</div>
			</body>
			</html>
		`)); err != nil {
			log.Printf("Failed to write callback response: %v", err)
		}
		codeChan <- code
	})

	// Use port 10999 to match GooglePhotosRedirectURI
	server := &http.Server{
		Addr:              ":10999",
		Handler:           mux,
		ReadHeaderTimeout: 3 * time.Second, // G112: Potential Slowloris Attack
	}

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errChan <- err
		}
	}()

	// 3. Construct Auth URL
	authURL, _ := url.Parse(GooglePhotosAuthURL)
	q := authURL.Query()
	q.Set("client_id", GoogleClientID)
	q.Set("redirect_uri", GooglePhotosRedirectURI)
	q.Set("response_type", "code")
	q.Set("scope", "https://www.googleapis.com/auth/photospicker.mediaitems.readonly")
	q.Set("code_challenge", challenge)
	q.Set("code_challenge_method", "S256")
	q.Set("state", state)
	q.Set("prompt", "consent")
	q.Set("access_type", "offline")
	authURL.RawQuery = q.Encode()

	// 4. Open Browser
	log.Printf("Opening browser for OAuth: %s", authURL.String())
	if err := openURLFunc(authURL); err != nil {
		server.Close()
		return fmt.Errorf("failed to open browser: %w", err)
	}

	// 5. Wait for Code
	var code string
	select {
	case code = <-codeChan:
	case err := <-errChan:
		server.Close()
		return err
	case <-time.After(2 * time.Minute):
		server.Close()
		return fmt.Errorf("authentication timed out")
	}

	// Shutdown server gracefully
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		log.Printf("Failed to shutdown callback server gracefully: %v", err)
	}

	// 6. Exchange Code for Token
	return a.exchangeCodeForToken(code, verifier)
}

func (a *Authenticator) exchangeCodeForToken(code, verifier string) error {
	data := url.Values{}
	data.Set("client_id", GoogleClientID)
	data.Set("client_secret", GoogleClientSecret) // Required for Desktop apps even with PKCE
	data.Set("code", code)
	data.Set("grant_type", "authorization_code")
	data.Set("redirect_uri", GooglePhotosRedirectURI)
	data.Set("code_verifier", verifier)

	resp, err := a.httpClient.PostForm(GooglePhotosTokenURL, data)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("token exchange failed with status: %d", resp.StatusCode)
	}

	var result struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return err
	}

	// Save tokens
	a.cfg.SetGooglePhotosToken(result.AccessToken)
	if result.RefreshToken != "" {
		a.cfg.SetGooglePhotosRefreshToken(result.RefreshToken)
	}
	// Calculate expiry
	expiry := time.Now().Add(time.Duration(result.ExpiresIn) * time.Second)
	a.cfg.SetGooglePhotosTokenExpiry(expiry)

	log.Print("Google Photos authentication successful.")
	return nil
}

// RefreshToken refreshes the access token if needed.
func (a *Authenticator) RefreshToken() error {
	refreshToken := a.cfg.GetGooglePhotosRefreshToken()
	if refreshToken == "" {
		return fmt.Errorf("no refresh token available")
	}

	data := url.Values{}
	data.Set("client_id", GoogleClientID)
	data.Set("client_secret", GoogleClientSecret)
	data.Set("refresh_token", refreshToken)
	data.Set("grant_type", "refresh_token")

	resp, err := a.httpClient.PostForm(GooglePhotosTokenURL, data)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("token refresh failed with status: %d", resp.StatusCode)
	}

	var result struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return err
	}

	a.cfg.SetGooglePhotosToken(result.AccessToken)
	expiry := time.Now().Add(time.Duration(result.ExpiresIn) * time.Second)
	a.cfg.SetGooglePhotosTokenExpiry(expiry)

	return nil
}

// EnsureValidToken checks if the current token is valid, and refreshes it if not.
func (a *Authenticator) EnsureValidToken() error {
	expiry := a.cfg.GetGooglePhotosTokenExpiry()
	// Refresh if expired or expiring within 5 minutes
	if time.Now().After(expiry.Add(-5 * time.Minute)) {
		return a.RefreshToken()
	}
	return nil
}

func generateRandomString(length int) (string, error) {
	b := make([]byte, length)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}
	// PKCE requires Base64URL encoding without padding
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func generateCodeChallenge(verifier string) string {
	h := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(h[:])
}

// RevokeToken invalidates the given access token or refresh token.
func (a *Authenticator) RevokeToken(token string) error {
	if token == "" {
		return nil
	}
	// Google revocation endpoint
	revokeURL := "https://oauth2.googleapis.com/revoke"

	// Prepare POST data
	data := url.Values{}
	data.Set("token", token)

	resp, err := a.httpClient.PostForm(revokeURL, data)
	if err != nil {
		return fmt.Errorf("failed to send revocation request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// Just log, don't fail hard if it's already invalid
		log.Printf("Token revocation returned status %d. It may already be invalid.", resp.StatusCode)
	}
	return nil
}
