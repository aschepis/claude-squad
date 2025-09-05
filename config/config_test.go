package config

import (
	"claude-squad/log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/go-git/go-git/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMain runs before all tests to set up the test environment
func TestMain(m *testing.M) {
	// Initialize the logger before any tests run
	log.Initialize(false)
	defer log.Close()

	exitCode := m.Run()
	os.Exit(exitCode)
}

func TestGetClaudeCommand(t *testing.T) {
	originalShell := os.Getenv("SHELL")
	originalPath := os.Getenv("PATH")
	defer func() {
		os.Setenv("SHELL", originalShell)
		os.Setenv("PATH", originalPath)
	}()

	t.Run("finds claude in PATH", func(t *testing.T) {
		// Create a temporary directory with a mock claude executable
		tempDir := t.TempDir()
		claudePath := filepath.Join(tempDir, "claude")

		// Create a mock executable
		err := os.WriteFile(claudePath, []byte("#!/bin/bash\necho 'mock claude'"), 0755)
		require.NoError(t, err)

		// Set PATH to include our temp directory
		os.Setenv("PATH", tempDir+":"+originalPath)
		os.Setenv("SHELL", "/bin/bash")

		result, err := GetClaudeCommand()

		assert.NoError(t, err)
		assert.True(t, strings.Contains(result, "claude"))
	})

	t.Run("handles missing claude command", func(t *testing.T) {
		// Set PATH to a directory that doesn't contain claude
		tempDir := t.TempDir()
		os.Setenv("PATH", tempDir)
		os.Setenv("SHELL", "/bin/bash")

		result, err := GetClaudeCommand()

		assert.Error(t, err)
		assert.Equal(t, "", result)
		assert.Contains(t, err.Error(), "claude command not found")
	})

	t.Run("handles empty SHELL environment", func(t *testing.T) {
		// Create a temporary directory with a mock claude executable
		tempDir := t.TempDir()
		claudePath := filepath.Join(tempDir, "claude")

		// Create a mock executable
		err := os.WriteFile(claudePath, []byte("#!/bin/bash\necho 'mock claude'"), 0755)
		require.NoError(t, err)

		// Set PATH and unset SHELL
		os.Setenv("PATH", tempDir+":"+originalPath)
		os.Unsetenv("SHELL")

		result, err := GetClaudeCommand()

		assert.NoError(t, err)
		assert.True(t, strings.Contains(result, "claude"))
	})

	t.Run("handles alias parsing", func(t *testing.T) {
		// Test core alias formats
		aliasRegex := regexp.MustCompile(`(?:aliased to|->|=)\s*([^\s]+)`)

		// Standard alias format
		output := "claude: aliased to /usr/local/bin/claude"
		matches := aliasRegex.FindStringSubmatch(output)
		assert.Len(t, matches, 2)
		assert.Equal(t, "/usr/local/bin/claude", matches[1])

		// Direct path (no alias)
		output = "/usr/local/bin/claude"
		matches = aliasRegex.FindStringSubmatch(output)
		assert.Len(t, matches, 0)
	})
}

func TestDefaultConfig(t *testing.T) {
	t.Run("creates config with default values", func(t *testing.T) {
		config := DefaultConfig()

		assert.NotNil(t, config)
		assert.NotEmpty(t, config.DefaultProgram)
		assert.False(t, config.AutoYes)
		assert.Equal(t, 1000, config.DaemonPollInterval)
		assert.NotEmpty(t, config.BranchPrefix)
		assert.True(t, strings.HasSuffix(config.BranchPrefix, "/"))
	})

}

func TestGetConfigDir(t *testing.T) {
	t.Run("returns valid config directory", func(t *testing.T) {
		configDir, err := GetConfigDir()

		assert.NoError(t, err)
		assert.NotEmpty(t, configDir)
		assert.True(t, strings.HasSuffix(configDir, ".claude-squad"))

		// Verify it's an absolute path
		assert.True(t, filepath.IsAbs(configDir))
	})
}

func TestLoadConfig(t *testing.T) {
	t.Run("returns default config when file doesn't exist", func(t *testing.T) {
		// Use a temporary home directory to avoid interfering with real config
		originalHome := os.Getenv("HOME")
		tempHome := t.TempDir()
		os.Setenv("HOME", tempHome)
		defer os.Setenv("HOME", originalHome)

		config := LoadConfig()

		assert.NotNil(t, config)
		assert.NotEmpty(t, config.DefaultProgram)
		assert.False(t, config.AutoYes)
		assert.Equal(t, 1000, config.DaemonPollInterval)
		assert.NotEmpty(t, config.BranchPrefix)
	})

	t.Run("loads valid config file", func(t *testing.T) {
		// Create a temporary config directory
		tempHome := t.TempDir()
		configDir := filepath.Join(tempHome, ".claude-squad")
		err := os.MkdirAll(configDir, 0755)
		require.NoError(t, err)

		// Create a test config file
		configPath := filepath.Join(configDir, ConfigFileName)
		configContent := `{
			"default_program": "test-claude",
			"auto_yes": true,
			"daemon_poll_interval": 2000,
			"branch_prefix": "test/"
		}`
		err = os.WriteFile(configPath, []byte(configContent), 0644)
		require.NoError(t, err)

		// Override HOME environment
		originalHome := os.Getenv("HOME")
		os.Setenv("HOME", tempHome)
		defer os.Setenv("HOME", originalHome)

		config := LoadConfig()

		assert.NotNil(t, config)
		assert.Equal(t, "test-claude", config.DefaultProgram)
		assert.True(t, config.AutoYes)
		assert.Equal(t, 2000, config.DaemonPollInterval)
		assert.Equal(t, "test/", config.BranchPrefix)
	})

	t.Run("returns default config on invalid JSON", func(t *testing.T) {
		// Create a temporary config directory
		tempHome := t.TempDir()
		configDir := filepath.Join(tempHome, ".claude-squad")
		err := os.MkdirAll(configDir, 0755)
		require.NoError(t, err)

		// Create an invalid config file
		configPath := filepath.Join(configDir, ConfigFileName)
		invalidContent := `{"invalid": json content}`
		err = os.WriteFile(configPath, []byte(invalidContent), 0644)
		require.NoError(t, err)

		// Override HOME environment
		originalHome := os.Getenv("HOME")
		os.Setenv("HOME", tempHome)
		defer os.Setenv("HOME", originalHome)

		config := LoadConfig()

		// Should return default config when JSON is invalid
		assert.NotNil(t, config)
		assert.NotEmpty(t, config.DefaultProgram)
		assert.False(t, config.AutoYes)                  // Default value
		assert.Equal(t, 1000, config.DaemonPollInterval) // Default value
	})
}

func TestSaveConfig(t *testing.T) {
	t.Run("saves config to file", func(t *testing.T) {
		// Create a temporary config directory
		tempHome := t.TempDir()

		// Override HOME environment
		originalHome := os.Getenv("HOME")
		os.Setenv("HOME", tempHome)
		defer os.Setenv("HOME", originalHome)

		// Create a test config
		testConfig := &Config{
			DefaultProgram:     "test-program",
			AutoYes:            true,
			DaemonPollInterval: 3000,
			BranchPrefix:       "test-branch/",
		}

		err := SaveConfig(testConfig)
		assert.NoError(t, err)

		// Verify the file was created
		configDir := filepath.Join(tempHome, ".claude-squad")
		configPath := filepath.Join(configDir, ConfigFileName)

		assert.FileExists(t, configPath)

		// Load and verify the content
		loadedConfig := LoadConfig()
		assert.Equal(t, testConfig.DefaultProgram, loadedConfig.DefaultProgram)
		assert.Equal(t, testConfig.AutoYes, loadedConfig.AutoYes)
		assert.Equal(t, testConfig.DaemonPollInterval, loadedConfig.DaemonPollInterval)
		assert.Equal(t, testConfig.BranchPrefix, loadedConfig.BranchPrefix)
	})
}

// Tests for the new per-project functionality

func TestSetUseProjects(t *testing.T) {
	// Save original state
	originalUseProjects := useProjectsMode
	defer func() { useProjectsMode = originalUseProjects }()

	t.Run("enables projects mode", func(t *testing.T) {
		SetUseProjects(true)
		assert.True(t, useProjectsMode)
	})

	t.Run("disables projects mode", func(t *testing.T) {
		SetUseProjects(false)
		assert.False(t, useProjectsMode)
	})
}

func TestGetStateDir(t *testing.T) {
	// Save original state
	originalUseProjects := useProjectsMode
	defer func() { useProjectsMode = originalUseProjects }()

	// Create temporary directories for testing
	tempHome := t.TempDir()

	// Override HOME environment
	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tempHome)
	defer os.Setenv("HOME", originalHome)

	t.Run("returns config dir in default mode", func(t *testing.T) {
		SetUseProjects(false)

		stateDir, err := GetStateDir()
		assert.NoError(t, err)

		configDir, err := GetConfigDir()
		require.NoError(t, err)

		assert.Equal(t, configDir, stateDir)
		assert.True(t, strings.HasSuffix(stateDir, ".claude-squad"))
	})

	t.Run("returns project-specific dir in projects mode", func(t *testing.T) {
		// Create a temporary git repository
		repoDir := t.TempDir()
		_, err := git.PlainInit(repoDir, false)
		require.NoError(t, err)

		// Change to the repo directory
		originalDir, err := os.Getwd()
		require.NoError(t, err)
		defer os.Chdir(originalDir)

		err = os.Chdir(repoDir)
		require.NoError(t, err)

		SetUseProjects(true)

		stateDir, err := GetStateDir()
		assert.NoError(t, err)

		configDir, err := GetConfigDir()
		require.NoError(t, err)

		// Should be in projects subdirectory
		assert.NotEqual(t, configDir, stateDir)
		assert.True(t, strings.HasPrefix(stateDir, configDir))
		assert.Contains(t, stateDir, "projects")

		// Should contain a project hash
		parts := strings.Split(stateDir, string(filepath.Separator))
		assert.True(t, len(parts) >= 2)
		projectHash := parts[len(parts)-1]
		assert.Len(t, projectHash, 16) // We use first 16 chars of SHA256
	})

	t.Run("returns same dir for same git repo", func(t *testing.T) {
		// Create a temporary git repository
		repoDir := t.TempDir()
		_, err := git.PlainInit(repoDir, false)
		require.NoError(t, err)

		// Create a subdirectory
		subDir := filepath.Join(repoDir, "subdir")
		err = os.MkdirAll(subDir, 0755)
		require.NoError(t, err)

		originalDir, err := os.Getwd()
		require.NoError(t, err)
		defer os.Chdir(originalDir)

		SetUseProjects(true)

		// Get state dir from repo root
		err = os.Chdir(repoDir)
		require.NoError(t, err)
		stateDir1, err := GetStateDir()
		assert.NoError(t, err)

		// Get state dir from subdirectory
		err = os.Chdir(subDir)
		require.NoError(t, err)
		stateDir2, err := GetStateDir()
		assert.NoError(t, err)

		// Should be the same
		assert.Equal(t, stateDir1, stateDir2)
	})

	t.Run("returns different dirs for different git repos", func(t *testing.T) {
		// Create two temporary git repositories
		repoDir1 := t.TempDir()
		_, err := git.PlainInit(repoDir1, false)
		require.NoError(t, err)

		repoDir2 := t.TempDir()
		_, err = git.PlainInit(repoDir2, false)
		require.NoError(t, err)

		originalDir, err := os.Getwd()
		require.NoError(t, err)
		defer os.Chdir(originalDir)

		SetUseProjects(true)

		// Get state dir from repo 1
		err = os.Chdir(repoDir1)
		require.NoError(t, err)
		stateDir1, err := GetStateDir()
		assert.NoError(t, err)

		// Get state dir from repo 2
		err = os.Chdir(repoDir2)
		require.NoError(t, err)
		stateDir2, err := GetStateDir()
		assert.NoError(t, err)

		// Should be different
		assert.NotEqual(t, stateDir1, stateDir2)
	})

	t.Run("fails when not in git repo in projects mode", func(t *testing.T) {
		// Create a non-git directory
		nonGitDir := t.TempDir()

		originalDir, err := os.Getwd()
		require.NoError(t, err)
		defer os.Chdir(originalDir)

		err = os.Chdir(nonGitDir)
		require.NoError(t, err)

		SetUseProjects(true)

		_, err = GetStateDir()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to find git repository root")
	})
}

func TestFindGitRepoRoot(t *testing.T) {
	t.Run("finds repo root from subdirectory", func(t *testing.T) {
		// Create a temporary git repository
		repoDir := t.TempDir()
		_, err := git.PlainInit(repoDir, false)
		require.NoError(t, err)

		// Create nested subdirectories
		subDir := filepath.Join(repoDir, "level1", "level2", "level3")
		err = os.MkdirAll(subDir, 0755)
		require.NoError(t, err)

		// Find repo root from nested subdirectory
		foundRoot, err := findGitRepoRoot(subDir)
		assert.NoError(t, err)
		assert.Equal(t, repoDir, foundRoot)
	})

	t.Run("finds repo root from repo root", func(t *testing.T) {
		// Create a temporary git repository
		repoDir := t.TempDir()
		_, err := git.PlainInit(repoDir, false)
		require.NoError(t, err)

		// Find repo root from repo root itself
		foundRoot, err := findGitRepoRoot(repoDir)
		assert.NoError(t, err)
		assert.Equal(t, repoDir, foundRoot)
	})

	t.Run("fails when not in git repo", func(t *testing.T) {
		// Create a non-git directory
		nonGitDir := t.TempDir()

		_, err := findGitRepoRoot(nonGitDir)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to find Git repository root")
	})
}
