package prepare

import (
	"strings"
	"testing"

	"github.com/google/go-containerregistry/pkg/name"
)

func TestParseImageRegistryAuth(t *testing.T) {
	auth, err := parseImageRegistryAuth(map[string]any{
		"auth": []any{
			map[string]any{
				"registry": "registry.example.com",
				"basic": map[string]any{
					"username": "robot",
					"password": "secret",
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("parseImageRegistryAuth failed: %v", err)
	}
	entry, ok := auth["registry.example.com"]
	if !ok {
		t.Fatalf("expected registry entry, got %v", auth)
	}
	if entry.username != "robot" || entry.password != "secret" {
		t.Fatalf("unexpected auth entry: %+v", entry)
	}
}

func TestParseImageRegistryAuthRejectsDuplicateRegistry(t *testing.T) {
	_, err := parseImageRegistryAuth(map[string]any{
		"auth": []any{
			map[string]any{"registry": "registry.example.com", "basic": map[string]any{"username": "a", "password": "b"}},
			map[string]any{"registry": "registry.example.com", "basic": map[string]any{"username": "c", "password": "d"}},
		},
	})
	if err == nil {
		t.Fatalf("expected duplicate registry error")
	}
	if want := "duplicate registry entries"; err != nil && !strings.Contains(err.Error(), want) {
		t.Fatalf("expected %q in error, got %v", want, err)
	}
}

func TestImageAuthKeychainUsesExplicitRegistryCredentials(t *testing.T) {
	ref, err := name.ParseReference("registry.example.com/team/app:1.0")
	if err != nil {
		t.Fatalf("parse reference: %v", err)
	}
	keychain := imageRegistryAuthMap{
		"registry.example.com": {registry: "registry.example.com", username: "robot", password: "secret"},
	}.keychain()
	auth, err := keychain.Resolve(ref.Context())
	if err != nil {
		t.Fatalf("resolve auth: %v", err)
	}
	config, err := auth.Authorization()
	if err != nil {
		t.Fatalf("authorization: %v", err)
	}
	if config.Username != "robot" || config.Password != "secret" {
		t.Fatalf("unexpected auth config: %+v", config)
	}
}

func TestImageAuthKeychainFallsBackWhenRegistryNotConfigured(t *testing.T) {
	ref, err := name.ParseReference("registry.k8s.io/pause:3.9")
	if err != nil {
		t.Fatalf("parse reference: %v", err)
	}
	keychain := imageRegistryAuthMap{
		"registry.example.com": {registry: "registry.example.com", username: "robot", password: "secret"},
	}.keychain()
	auth, err := keychain.Resolve(ref.Context())
	if err != nil {
		t.Fatalf("resolve auth: %v", err)
	}
	config, err := auth.Authorization()
	if err != nil {
		t.Fatalf("authorization: %v", err)
	}
	if config.Username == "robot" {
		t.Fatalf("expected fallback auth, got explicit credentials: %+v", config)
	}
}
