package fiberoapi

import (
	"testing"

	"github.com/gofiber/fiber/v2"
)

func TestConfigMerging(t *testing.T) {
	app := fiber.New()

	t.Run("Default config when no config provided", func(t *testing.T) {
		oapi := New(app)
		config := oapi.Config()
		
		// Should use defaults
		if !config.EnableValidation {
			t.Error("Expected EnableValidation to be true by default")
		}
		if !config.EnableOpenAPIDocs {
			t.Error("Expected EnableOpenAPIDocs to be true by default")
		}
		if config.EnableAuthorization {
			t.Error("Expected EnableAuthorization to be false by default")
		}
	})

	t.Run("Empty config preserves defaults", func(t *testing.T) {
		oapi := New(app, Config{})
		config := oapi.Config()
		
		// Should preserve defaults when empty config is provided
		if !config.EnableValidation {
			t.Error("Expected EnableValidation to remain true with empty config")
		}
		if !config.EnableOpenAPIDocs {
			t.Error("Expected EnableOpenAPIDocs to remain true with empty config")
		}
		if config.EnableAuthorization {
			t.Error("Expected EnableAuthorization to remain false with empty config")
		}
	})

	t.Run("Explicit config overrides defaults", func(t *testing.T) {
		// Provide explicit config with some non-default values
		oapi := New(app, Config{
			EnableValidation:    false, // Explicitly set to false
			EnableOpenAPIDocs:   false, // Explicitly set to false  
			EnableAuthorization: true,  // This indicates explicit configuration
		})
		config := oapi.Config()
		
		// Should respect explicit values
		if config.EnableValidation {
			t.Error("Expected EnableValidation to be false when explicitly set")
		}
		if config.EnableOpenAPIDocs {
			t.Error("Expected EnableOpenAPIDocs to be false when explicitly set")
		}
		if !config.EnableAuthorization {
			t.Error("Expected EnableAuthorization to be true when explicitly set")
		}
	})

	t.Run("Partial explicit config", func(t *testing.T) {
		// Provide config with auth service (indicates explicit configuration)
		authService := &struct{ AuthorizationService }{}
		oapi := New(app, Config{
			EnableValidation:  false, // Should be respected
			EnableOpenAPIDocs: true,  // Should be respected
			AuthService:       authService,
		})
		config := oapi.Config()
		
		// Should respect explicit values when auth service is provided
		if config.EnableValidation {
			t.Error("Expected EnableValidation to be false when explicitly configured")
		}
		if !config.EnableOpenAPIDocs {
			t.Error("Expected EnableOpenAPIDocs to be true when explicitly configured")
		}
		if config.AuthService == nil {
			t.Error("Expected AuthService to be set")
		}
	})
}