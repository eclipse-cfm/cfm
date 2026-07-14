package system

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const appName = "testapp"

func TestLoadConfigFromEnvironment(t *testing.T) {
	// setup
	t.Setenv("TESTAPP_FOO", "foo_value")
	t.Setenv("TESTAPP_BAR", "bar_value")

	v := LoadConfigOrPanic(appName)

	assert.Equal(t, "foo_value", v.GetString("FOO"))
	assert.Equal(t, "bar_value", v.GetString("bar"))
}

//nolint:errcheck
func TestLoadConfigFromFile(t *testing.T) {
	// setup
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "testapp.yaml")
	configContent := []byte(`
foo: "foo_value"
bar: "bar_value"
`)
	err := os.WriteFile(configFile, configContent, 0644)
	require.NoError(t, err)

	// create symlink to make the config accessible in the current directory
	err = os.Symlink(configFile, "./testapp.yaml")
	require.NoError(t, err)
	defer os.Remove("./testapp.yaml")

	v := LoadConfigOrPanic(appName)

	assert.Equal(t, "foo_value", v.GetString("foo"))
	assert.Equal(t, "bar_value", v.GetString("bar"))
}

func TestLoadConfigDirOverride(t *testing.T) {
	// a config file in the current directory, which must be ignored when the override is set
	cwdContent := []byte(`foo: "cwd_value"`)
	require.NoError(t, os.WriteFile("./testapp.yaml", cwdContent, 0644))
	defer func() { _ = os.Remove("./testapp.yaml") }()

	// a config file in the override directory
	overrideDir := t.TempDir()
	overrideContent := []byte(`foo: "override_value"`)
	require.NoError(t, os.WriteFile(filepath.Join(overrideDir, "testapp.yaml"), overrideContent, 0644))

	t.Setenv(ConfigDirEnvVar, overrideDir)
	v := LoadConfigOrPanic(appName)
	assert.Equal(t, "override_value", v.GetString("foo"), "only the override directory must be searched")

	// pointing the override at an empty directory must yield no file-based configuration at all
	t.Setenv(ConfigDirEnvVar, t.TempDir())
	v = LoadConfigOrPanic(appName)
	assert.Empty(t, v.GetString("foo"), "config files outside the override directory must be ignored")
}

//nolint:errcheck
func TestLoadConfigEnvPriorityOverFile(t *testing.T) {
	// create the config file
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "testapp.yaml")
	configContent := []byte(`
foo: "foo_value_file"
bar: "bar_value_file"
`)
	err := os.WriteFile(configFile, configContent, 0644)
	require.NoError(t, err)

	// Create symlink to make the config accessible in current directory
	err = os.Symlink(configFile, "./testapp.yaml")
	require.NoError(t, err)
	defer os.Remove("./testapp.yaml")

	// Set environment variables
	t.Setenv("TESTAPP_FOO", "foo_value_file")
	t.Setenv("TESTAPP_BAR", "bar_value_env")

	v := LoadConfigOrPanic(appName)

	assert.Equal(t, "foo_value_file", v.GetString("foo"))
	assert.Equal(t, "bar_value_env", v.GetString("bar"))
}
