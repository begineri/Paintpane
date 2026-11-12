//go:build release

package ui

import "github.com/dixieflatline76/Spice/v2/pkg/api"

// RegisterDebugHandlers does nothing in release builds; see debug_dev.go.
func RegisterDebugHandlers(*api.Server) {}
