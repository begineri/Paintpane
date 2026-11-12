package ui

import (
	"errors"
	"net/url"
	"testing"

	"fyne.io/fyne/v2"
	"github.com/stretchr/testify/assert"
)

// urlApp records what the app is asked to open.
type urlApp struct {
	fyne.App
	calls []string
	err   error
}

func (a *urlApp) OpenURL(u *url.URL) error {
	a.calls = append(a.calls, "open "+u.String())
	return a.err
}

// Links the tray opens let the browser come to the front first (2026-11-12): the menu has
// closed by then, so without it the page opened behind the other windows.
func TestOpenURLLetsTheBrowserComeForwardFirst(t *testing.T) {
	fake := &urlApp{}
	old := allowForeground
	defer func() { allowForeground = old }()
	allowForeground = func() bool {
		fake.calls = append(fake.calls, "allow")
		return true
	}
	sa := &SpiceApp{App: fake}
	u, _ := url.Parse("https://clevelandart.org/art/2024.27")

	assert.NoError(t, sa.OpenURL(u))
	assert.Equal(t, []string{"allow", "open https://clevelandart.org/art/2024.27"}, fake.calls)

	// When the right cannot be passed on, the page still opens; an error is passed back.
	fake.calls, fake.err = nil, errors.New("no browser")
	allowForeground = func() bool { fake.calls = append(fake.calls, "allow"); return false }
	assert.EqualError(t, sa.OpenURL(u), "no browser")
	assert.Equal(t, []string{"allow", "open https://clevelandart.org/art/2024.27"}, fake.calls)
}
