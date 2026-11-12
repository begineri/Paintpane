package googlephotos

import (
	"os"
	"testing"

	"github.com/dixieflatline76/Spice/v2/pkg/i18n"
	"github.com/zalando/go-keyring"
)

// TestMain pins the language because tests assert English UI strings,
// and i18n otherwise follows the OS locale of whoever runs them. The credential store is mocked
// so a test never reads the API keys of whoever runs it.
func TestMain(m *testing.M) {
	i18n.SetLanguage("en")
	keyring.MockInit()
	os.Exit(m.Run())
}
