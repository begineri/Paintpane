package wallpaper

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dixieflatline76/Spice/v2/pkg/curation"
)

func TestCuratedSelectAllListsEveryCollection(t *testing.T) {
	p := new(MockImageProvider)
	p.On("ID").Return("testprov")
	entries := []curation.CollectionEntry{{Key: "a"}, {Key: "b"}, {Key: "c"}}

	item := curatedSelectAll(p, entries)
	assert.Equal(t, "Select all", item.Label)
	assert.Equal(t, []string{"testprov_a", "testprov_b", "testprov_c"}, item.Members)
}
