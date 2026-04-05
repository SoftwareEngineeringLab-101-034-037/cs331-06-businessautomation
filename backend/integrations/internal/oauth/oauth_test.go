package oauth

import (
	"encoding/base64"
	"encoding/json"
	"testing"

	"golang.org/x/oauth2"
)

func TestNormalizeProviderID(t *testing.T) {
	if got := normalizeProviderID("Google-Forms"); got != providerGoogleForms {
		t.Fatalf("unexpected provider mapping: %q", got)
	}
	if got := normalizeProviderID("gmail"); got != providerGmail {
		t.Fatalf("unexpected provider mapping: %q", got)
	}
	if got := normalizeProviderID("other"); got != "" {
		t.Fatalf("expected unknown provider to map to empty")
	}
}

func TestRequiredScopesForProviderIncludesIdentity(t *testing.T) {
	scopes := requiredScopesForProvider(providerGoogleForms)
	if len(scopes) < len(identityScopes) {
		t.Fatalf("expected identity scopes included")
	}
}

func TestHasRequiredScopes(t *testing.T) {
	ok, missing := hasRequiredScopes([]string{"a", "b"}, []string{"a"})
	if !ok || len(missing) != 0 {
		t.Fatalf("expected scope check to pass")
	}
	ok, missing = hasRequiredScopes([]string{"a"}, []string{"a", "b"})
	if ok || len(missing) != 1 || missing[0] != "b" {
		t.Fatalf("expected missing scope b, got %#v", missing)
	}
}

func TestGenerateAndValidateState(t *testing.T) {
	svc := NewService("id", "secret", "http://localhost/callback", nil)
	state, err := svc.GenerateState("org_1", "user_1", providerGmail)
	if err != nil {
		t.Fatalf("GenerateState failed: %v", err)
	}
	org, user, provider, err := svc.ValidateAndConsumeState(state)
	if err != nil {
		t.Fatalf("ValidateAndConsumeState failed: %v", err)
	}
	if org != "org_1" || user != "user_1" || provider != providerGmail {
		t.Fatalf("unexpected decoded state values: %q %q %q", org, user, provider)
	}
}

func TestValidateStateRejectsTamperedPayload(t *testing.T) {
	svc := NewService("id", "secret", "http://localhost/callback", nil)
	state, err := svc.GenerateState("org_1", "user_1")
	if err != nil {
		t.Fatalf("GenerateState failed: %v", err)
	}
	tampered := state + "x"
	if _, _, _, err := svc.ValidateAndConsumeState(tampered); err == nil {
		t.Fatalf("expected tampered state to fail")
	}
}

func TestParseTokenScopes(t *testing.T) {
	tok := (&oauth2.Token{}).WithExtra(map[string]interface{}{"scope": "a b a"})
	scopes := parseTokenScopes(tok)
	if len(scopes) != 2 {
		t.Fatalf("expected deduped scopes, got %#v", scopes)
	}
}

func TestExtractBearerUserID(t *testing.T) {
	payload, _ := json.Marshal(map[string]interface{}{"sub": "user_123"})
	token := "x." + base64.RawURLEncoding.EncodeToString(payload) + ".y"
	user, err := extractBearerUserID("Bearer " + token)
	if err != nil {
		t.Fatalf("extractBearerUserID failed: %v", err)
	}
	if user != "user_123" {
		t.Fatalf("expected user_123, got %q", user)
	}
}

func TestIsRefreshReauthError(t *testing.T) {
	if !isRefreshReauthError(errString("unauthorized_client")) {
		t.Fatalf("expected unauthorized_client to require reconnect")
	}
	if !isRefreshReauthError(errString("invalid_grant")) {
		t.Fatalf("expected invalid_grant to require reconnect")
	}
	if isRefreshReauthError(nil) {
		t.Fatalf("nil error must not match")
	}
}

type errString string

func (e errString) Error() string { return string(e) }
