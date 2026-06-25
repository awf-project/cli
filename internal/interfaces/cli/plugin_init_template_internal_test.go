package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCurrentAWFModuleVersion_UsesPseudoVersionForLocalSourceCheckout(t *testing.T) {
	version, err := currentAWFModuleVersion(true)

	require.NoError(t, err)
	assert.Equal(t, "v0.0.0", version)
}

func TestCurrentAWFModuleVersion_UsesReleasedCLIVersionWhenNoLocalSourceCheckoutExists(t *testing.T) {
	originalVersion := Version
	Version = "0.10.1"
	t.Cleanup(func() { Version = originalVersion })

	version, err := currentAWFModuleVersion(false)

	require.NoError(t, err)
	assert.Equal(t, "v0.10.1", version)
}

func TestCurrentAWFModuleVersion_RejectsDevelopmentVersionWithoutLocalSourceCheckout(t *testing.T) {
	originalVersion := Version
	Version = "dev"
	t.Cleanup(func() { Version = originalVersion })

	_, err := currentAWFModuleVersion(false)

	require.Error(t, err)
	assert.ErrorContains(t, err, "source checkout")
	assert.ErrorContains(t, err, "released awf binary")
}
