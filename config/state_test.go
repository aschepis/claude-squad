package config

import (
	"claude-squad/log"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-git/go-git/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStateIsolation(t *testing.T) {
	// Save original state
	originalUseProjects := useProjectsMode
	defer func() { useProjectsMode = originalUseProjects }()

	// Initialize logger for tests
	log.Initialize(false)
	defer log.Close()

	t.Run("state isolation between projects", func(t *testing.T) {
		// Create temporary home directory
		tempHome := t.TempDir()
		originalHome := os.Getenv("HOME")
		os.Setenv("HOME", tempHome)
		defer os.Setenv("HOME", originalHome)

		// Create two separate git repositories
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

		// Create state for project 1
		err = os.Chdir(repoDir1)
		require.NoError(t, err)

		state1 := &State{
			HelpScreensSeen: 123,
			InstancesData:   json.RawMessage(`[{"title":"project1-instance"}]`),
		}
		err = SaveState(state1)
		assert.NoError(t, err)

		// Create different state for project 2
		err = os.Chdir(repoDir2)
		require.NoError(t, err)

		state2 := &State{
			HelpScreensSeen: 456,
			InstancesData:   json.RawMessage(`[{"title":"project2-instance"}]`),
		}
		err = SaveState(state2)
		assert.NoError(t, err)

		// Verify project 1 state is preserved
		err = os.Chdir(repoDir1)
		require.NoError(t, err)

		loadedState1 := LoadState()
		assert.Equal(t, uint32(123), loadedState1.HelpScreensSeen)
		assert.Contains(t, string(loadedState1.InstancesData), "project1-instance")
		assert.NotContains(t, string(loadedState1.InstancesData), "project2-instance")

		// Verify project 2 state is preserved
		err = os.Chdir(repoDir2)
		require.NoError(t, err)

		loadedState2 := LoadState()
		assert.Equal(t, uint32(456), loadedState2.HelpScreensSeen)
		assert.Contains(t, string(loadedState2.InstancesData), "project2-instance")
		assert.NotContains(t, string(loadedState2.InstancesData), "project1-instance")
	})

	t.Run("backwards compatibility - default mode uses original location", func(t *testing.T) {
		// Create temporary home directory
		tempHome := t.TempDir()
		originalHome := os.Getenv("HOME")
		os.Setenv("HOME", tempHome)
		defer os.Setenv("HOME", originalHome)

		// Create a git repository (to satisfy claude-squad's git requirement)
		repoDir := t.TempDir()
		_, err := git.PlainInit(repoDir, false)
		require.NoError(t, err)

		originalDir, err := os.Getwd()
		require.NoError(t, err)
		defer os.Chdir(originalDir)

		err = os.Chdir(repoDir)
		require.NoError(t, err)

		// Test default mode (projects disabled)
		SetUseProjects(false)

		defaultState := &State{
			HelpScreensSeen: 789,
			InstancesData:   json.RawMessage(`[{"title":"default-instance"}]`),
		}
		err = SaveState(defaultState)
		assert.NoError(t, err)

		// Verify it saves to the expected location
		configDir, err := GetConfigDir()
		require.NoError(t, err)
		expectedPath := filepath.Join(configDir, StateFileName)
		assert.FileExists(t, expectedPath)

		// Verify we can load it back
		loadedState := LoadState()
		assert.Equal(t, uint32(789), loadedState.HelpScreensSeen)
		assert.Contains(t, string(loadedState.InstancesData), "default-instance")

		// Switch to projects mode and verify it creates separate state
		SetUseProjects(true)

		projectState := &State{
			HelpScreensSeen: 999,
			InstancesData:   json.RawMessage(`[{"title":"project-instance"}]`),
		}
		err = SaveState(projectState)
		assert.NoError(t, err)

		loadedProjectState := LoadState()
		assert.Equal(t, uint32(999), loadedProjectState.HelpScreensSeen)
		assert.Contains(t, string(loadedProjectState.InstancesData), "project-instance")

		// Switch back to default mode and verify original state is preserved
		SetUseProjects(false)

		loadedDefaultState := LoadState()
		assert.Equal(t, uint32(789), loadedDefaultState.HelpScreensSeen)
		assert.Contains(t, string(loadedDefaultState.InstancesData), "default-instance")
		assert.NotContains(t, string(loadedDefaultState.InstancesData), "project-instance")
	})

	t.Run("state consistency within same project from different subdirectories", func(t *testing.T) {
		// Create temporary home directory
		tempHome := t.TempDir()
		originalHome := os.Getenv("HOME")
		os.Setenv("HOME", tempHome)
		defer os.Setenv("HOME", originalHome)

		// Create git repository with nested subdirectories
		repoDir := t.TempDir()
		_, err := git.PlainInit(repoDir, false)
		require.NoError(t, err)

		subDir1 := filepath.Join(repoDir, "src", "components")
		err = os.MkdirAll(subDir1, 0755)
		require.NoError(t, err)

		subDir2 := filepath.Join(repoDir, "tests", "unit")
		err = os.MkdirAll(subDir2, 0755)
		require.NoError(t, err)

		originalDir, err := os.Getwd()
		require.NoError(t, err)
		defer os.Chdir(originalDir)

		SetUseProjects(true)

		// Save state from first subdirectory
		err = os.Chdir(subDir1)
		require.NoError(t, err)

		state := &State{
			HelpScreensSeen: 555,
			InstancesData:   json.RawMessage(`[{"title":"shared-instance"}]`),
		}
		err = SaveState(state)
		assert.NoError(t, err)

		// Load state from different subdirectory
		err = os.Chdir(subDir2)
		require.NoError(t, err)

		loadedState := LoadState()
		assert.Equal(t, uint32(555), loadedState.HelpScreensSeen)
		assert.Contains(t, string(loadedState.InstancesData), "shared-instance")

		// Modify state from second subdirectory
		loadedState.HelpScreensSeen = 666
		loadedState.InstancesData = json.RawMessage(`[{"title":"updated-instance"}]`)
		err = SaveState(loadedState)
		assert.NoError(t, err)

		// Verify changes are visible from first subdirectory
		err = os.Chdir(subDir1)
		require.NoError(t, err)

		finalState := LoadState()
		assert.Equal(t, uint32(666), finalState.HelpScreensSeen)
		assert.Contains(t, string(finalState.InstancesData), "updated-instance")
		assert.NotContains(t, string(finalState.InstancesData), "shared-instance")
	})
}

func TestStateInterface(t *testing.T) {
	// Save original state
	originalUseProjects := useProjectsMode
	defer func() { useProjectsMode = originalUseProjects }()

	// Initialize logger for tests
	log.Initialize(false)
	defer log.Close()

	t.Run("state implements InstanceStorage interface", func(t *testing.T) {
		// Create temporary home directory
		tempHome := t.TempDir()
		originalHome := os.Getenv("HOME")
		os.Setenv("HOME", tempHome)
		defer os.Setenv("HOME", originalHome)

		// Create git repository
		repoDir := t.TempDir()
		_, err := git.PlainInit(repoDir, false)
		require.NoError(t, err)

		originalDir, err := os.Getwd()
		require.NoError(t, err)
		defer os.Chdir(originalDir)

		err = os.Chdir(repoDir)
		require.NoError(t, err)

		SetUseProjects(true)

		state := LoadState()

		// Test SaveInstances
		testData := json.RawMessage(`[{"title":"test-instance","path":"/test"}]`)
		err = state.SaveInstances(testData)
		assert.NoError(t, err)

		// Test GetInstances
		retrievedData := state.GetInstances()
		assert.JSONEq(t, string(testData), string(retrievedData))

		// Test DeleteAllInstances
		err = state.DeleteAllInstances()
		assert.NoError(t, err)

		retrievedData = state.GetInstances()
		assert.JSONEq(t, `[]`, string(retrievedData))
	})

	t.Run("state implements AppState interface", func(t *testing.T) {
		// Create temporary home directory
		tempHome := t.TempDir()
		originalHome := os.Getenv("HOME")
		os.Setenv("HOME", tempHome)
		defer os.Setenv("HOME", originalHome)

		// Create git repository
		repoDir := t.TempDir()
		_, err := git.PlainInit(repoDir, false)
		require.NoError(t, err)

		originalDir, err := os.Getwd()
		require.NoError(t, err)
		defer os.Chdir(originalDir)

		err = os.Chdir(repoDir)
		require.NoError(t, err)

		SetUseProjects(false) // Test default mode

		state := LoadState()

		// Test GetHelpScreensSeen (initial value)
		seen := state.GetHelpScreensSeen()
		assert.Equal(t, uint32(0), seen)

		// Test SetHelpScreensSeen
		err = state.SetHelpScreensSeen(12345)
		assert.NoError(t, err)

		// Test GetHelpScreensSeen (after setting)
		seen = state.GetHelpScreensSeen()
		assert.Equal(t, uint32(12345), seen)

		// Verify persistence by loading fresh state
		freshState := LoadState()
		seen = freshState.GetHelpScreensSeen()
		assert.Equal(t, uint32(12345), seen)
	})
}
