package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/github"
	"golang.org/x/oauth2/google"
)

// OAuthProvider encapsulates a provider's OAuth2 config and user-info URL.
type OAuthProvider struct {
	Config      *oauth2.Config
	UserInfoURL string
}

// NewOAuthProvider builds the oauth2.Config for the given provider name.
// baseURL is the public server URL used to construct the callback URL.
func NewOAuthProvider(provider, clientID, clientSecret, baseURL string) (*OAuthProvider, error) {
	callbackURL := baseURL + "/-/auth/callback"
	switch provider {
	case "google":
		return &OAuthProvider{
			Config: &oauth2.Config{
				ClientID:     clientID,
				ClientSecret: clientSecret,
				RedirectURL:  callbackURL,
				Scopes:       []string{"openid", "email", "profile"},
				Endpoint:     google.Endpoint,
			},
			UserInfoURL: "https://www.googleapis.com/oauth2/v3/userinfo",
		}, nil
	case "github":
		return &OAuthProvider{
			Config: &oauth2.Config{
				ClientID:     clientID,
				ClientSecret: clientSecret,
				RedirectURL:  callbackURL,
				Scopes:       []string{"read:user", "user:email"},
				Endpoint:     github.Endpoint,
			},
			UserInfoURL: "https://api.github.com/user",
		}, nil
	default:
		return nil, fmt.Errorf("unknown OAuth provider: %q", provider)
	}
}

// OAuthUserInfo holds the normalised user information returned by a provider.
type OAuthUserInfo struct {
	ProviderID string // subject / user id at the provider
	Email      string
	Login      string // username or display name
}

// FetchUserInfo exchanges an access token for the user's profile.
func (p *OAuthProvider) FetchUserInfo(ctx context.Context, accessToken string) (*OAuthUserInfo, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.UserInfoURL, nil)
	if err != nil {
		return nil, fmt.Errorf("build userinfo request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch userinfo: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 32*1024))
	if err != nil {
		return nil, fmt.Errorf("read userinfo body: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("userinfo endpoint returned %d: %s", resp.StatusCode, body)
	}

	// Both Google and GitHub return a JSON object; field names differ.
	var raw map[string]interface{}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("decode userinfo: %w", err)
	}

	info := &OAuthUserInfo{}

	// Google: sub, email, name
	// GitHub: id (number), email, login
	switch {
	case raw["sub"] != nil: // Google
		info.ProviderID = fmt.Sprintf("%v", raw["sub"])
		info.Email, _ = raw["email"].(string)
		info.Login, _ = raw["name"].(string)
	case raw["login"] != nil: // GitHub
		info.ProviderID = fmt.Sprintf("%v", raw["id"])
		info.Email, _ = raw["email"].(string)
		info.Login, _ = raw["login"].(string)
	default:
		return nil, fmt.Errorf("unrecognised userinfo response format")
	}

	return info, nil
}

// AuthCodeURL generates the provider redirect URL with the state nonce embedded.
func (p *OAuthProvider) AuthCodeURL(state string) string {
	return p.Config.AuthCodeURL(state, oauth2.AccessTypeOnline)
}

// Exchange converts an authorization code into a token.
func (p *OAuthProvider) Exchange(ctx context.Context, code string) (string, error) {
	tok, err := p.Config.Exchange(ctx, code)
	if err != nil {
		return "", fmt.Errorf("oauth exchange: %w", err)
	}
	return tok.AccessToken, nil
}
