// Package conformance provides test helpers for Fabric database plugins.
package conformance

import (
	"context"
	"fmt"

	db "tounilab.com/fabric/db/v1"
	"tounilab.com/fabric/db/v1/plugin"
)

// CheckFactory verifies that a plugin factory has a usable name and returns a Fabric DB.
//
// Plugin authors can call this from their own tests to catch registry contract
// mistakes before integrating with db.NewDB.
func CheckFactory(ctx context.Context, factory plugin.DriverFactory, cfg db.DBConfig) error {
	if factory == nil {
		return fmt.Errorf("plugin conformance: factory is nil")
	}
	if factory.Name() == "" {
		return fmt.Errorf("plugin conformance: factory name is empty")
	}
	if cfg == nil {
		return fmt.Errorf("plugin conformance: config is nil")
	}
	if cfg.Driver() == "" {
		return fmt.Errorf("plugin conformance: config driver is empty")
	}
	if factory.Name() != cfg.Driver() {
		return fmt.Errorf(
			"plugin conformance: factory name %q does not match config driver %q",
			factory.Name(),
			cfg.Driver(),
		)
	}
	result, err := factory.Create(ctx, cfg)
	if err != nil {
		return fmt.Errorf("plugin conformance: factory create failed: %w", err)
	}
	database, ok := result.(db.DB)
	if !ok {
		return fmt.Errorf("plugin conformance: factory returned %T, expected db.DB", result)
	}
	if err := database.Close(); err != nil {
		return fmt.Errorf("plugin conformance: close failed: %w", err)
	}
	return nil
}
