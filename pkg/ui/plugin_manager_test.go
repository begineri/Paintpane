package ui

import (
	"reflect"
	"testing"
)

// Prototype v35: Paintpane sends no Windows notifications, so plugins have no way to send one.
func TestPluginManagerHasNoNotifications(t *testing.T) {
	pm := reflect.TypeOf((*PluginManager)(nil)).Elem()
	for _, name := range []string{"NotifyUser", "RegisterNotifier"} {
		if _, ok := pm.MethodByName(name); ok {
			t.Errorf("PluginManager.%s is back; the app does not send notifications (docs/ui_refresh.md §12, v35)", name)
		}
	}
}
