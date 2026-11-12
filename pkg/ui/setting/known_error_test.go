package setting

import (
	"errors"
	"fmt"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestClassify(t *testing.T) {
	inner := errors.New("invalid Pexels API key")
	tagged := Known(ErrorInvalidKey, "Pexels", inner)

	kind, source := Classify(fmt.Errorf("verifying: %w", tagged))
	assert.Equal(t, ErrorInvalidKey, kind)
	assert.Equal(t, "Pexels", source)
	assert.Equal(t, "invalid Pexels API key", tagged.Error(), "the original text is unchanged")
	assert.ErrorIs(t, tagged, inner)

	// A network failure is recognised even when nothing tagged it.
	kind, _ = Classify(fmt.Errorf("making request: %w", &url.Error{Op: "Get", URL: "https://x", Err: errors.New("dial")}))
	assert.Equal(t, ErrorNetwork, kind)

	kind, _ = Classify(errors.New("something else"))
	assert.Equal(t, ErrorOther, kind)
	kind, _ = Classify(nil)
	assert.Equal(t, ErrorOther, kind)
	assert.Nil(t, Known(ErrorInvalidKey, "x", nil))
}
