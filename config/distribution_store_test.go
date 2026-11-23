//go:build appstore || msstore

package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsStoreDistribution_StoreBuild(t *testing.T) {
	assert.True(t, IsStoreDistribution(), "A build with appstore or msstore is a store distribution")
}
