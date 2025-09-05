package config

import (
	"claude-squad/log"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/go-git/go-git/v5"
)

const (
	StateFileName     = "state.json"
	InstancesFileName = "instances.json"
	ProjectsDir       = "projects"
)

var (
	useProjectsMode = false
)

// SetUseProjects enables or disables per-project state mode
func SetUseProjects(enable bool) {
	useProjectsMode = enable
}

// GetStateDir returns the directory where state should be stored
// In projects mode, it creates a subdirectory based on the git repo root
func GetStateDir() (string, error) {
	configDir, err := GetConfigDir()
	if err != nil {
		return "", err
	}

	if !useProjectsMode {
		return configDir, nil
	}

	// Find git repository root
	currentDir, err := filepath.Abs(".")
	if err != nil {
		return "", fmt.Errorf("failed to get current directory: %w", err)
	}

	repoRoot, err := findGitRepoRoot(currentDir)
	if err != nil {
		return "", fmt.Errorf("failed to find git repository root: %w", err)
	}

	// Generate project hash from repo root path
	hash := sha256.Sum256([]byte(repoRoot))
	projectHash := hex.EncodeToString(hash[:])[:16] // Use first 16 characters

	projectDir := filepath.Join(configDir, ProjectsDir, projectHash)
	return projectDir, nil
}

// findGitRepoRoot finds the root directory of the git repository
func findGitRepoRoot(path string) (string, error) {
	currentPath := path
	for {
		_, err := git.PlainOpen(currentPath)
		if err == nil {
			// Found the repository root
			return currentPath, nil
		}

		parent := filepath.Dir(currentPath)
		if parent == currentPath {
			// Reached the filesystem root without finding a repository
			return "", fmt.Errorf("failed to find Git repository root from path: %s", path)
		}
		currentPath = parent
	}
}

// InstanceStorage handles instance-related operations
type InstanceStorage interface {
	// SaveInstances saves the raw instance data
	SaveInstances(instancesJSON json.RawMessage) error
	// GetInstances returns the raw instance data
	GetInstances() json.RawMessage
	// DeleteAllInstances removes all stored instances
	DeleteAllInstances() error
}

// AppState handles application-level state
type AppState interface {
	// GetHelpScreensSeen returns the bitmask of seen help screens
	GetHelpScreensSeen() uint32
	// SetHelpScreensSeen updates the bitmask of seen help screens
	SetHelpScreensSeen(seen uint32) error
}

// StateManager combines instance storage and app state management
type StateManager interface {
	InstanceStorage
	AppState
}

// State represents the application state that persists between sessions
type State struct {
	// HelpScreensSeen is a bitmask tracking which help screens have been shown
	HelpScreensSeen uint32 `json:"help_screens_seen"`
	// Instances stores the serialized instance data as raw JSON
	InstancesData json.RawMessage `json:"instances"`
}

// DefaultState returns the default state
func DefaultState() *State {
	return &State{
		HelpScreensSeen: 0,
		InstancesData:   json.RawMessage("[]"),
	}
}

// LoadState loads the state from disk. If it cannot be done, we return the default state.
func LoadState() *State {
	stateDir, err := GetStateDir()
	if err != nil {
		log.ErrorLog.Printf("failed to get state directory: %v", err)
		return DefaultState()
	}

	statePath := filepath.Join(stateDir, StateFileName)
	data, err := os.ReadFile(statePath)
	if err != nil {
		if os.IsNotExist(err) {
			// Create and save default state if file doesn't exist
			defaultState := DefaultState()
			if saveErr := SaveState(defaultState); saveErr != nil {
				log.WarningLog.Printf("failed to save default state: %v", saveErr)
			}
			return defaultState
		}

		log.WarningLog.Printf("failed to get state file: %v", err)
		return DefaultState()
	}

	var state State
	if err := json.Unmarshal(data, &state); err != nil {
		log.ErrorLog.Printf("failed to parse state file: %v", err)
		return DefaultState()
	}

	return &state
}

// SaveState saves the state to disk
func SaveState(state *State) error {
	stateDir, err := GetStateDir()
	if err != nil {
		return fmt.Errorf("failed to get state directory: %w", err)
	}

	if err := os.MkdirAll(stateDir, 0755); err != nil {
		return fmt.Errorf("failed to create state directory: %w", err)
	}

	statePath := filepath.Join(stateDir, StateFileName)
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal state: %w", err)
	}

	return os.WriteFile(statePath, data, 0644)
}

// InstanceStorage interface implementation

// SaveInstances saves the raw instance data
func (s *State) SaveInstances(instancesJSON json.RawMessage) error {
	s.InstancesData = instancesJSON
	return SaveState(s)
}

// GetInstances returns the raw instance data
func (s *State) GetInstances() json.RawMessage {
	return s.InstancesData
}

// DeleteAllInstances removes all stored instances
func (s *State) DeleteAllInstances() error {
	s.InstancesData = json.RawMessage("[]")
	return SaveState(s)
}

// AppState interface implementation

// GetHelpScreensSeen returns the bitmask of seen help screens
func (s *State) GetHelpScreensSeen() uint32 {
	return s.HelpScreensSeen
}

// SetHelpScreensSeen updates the bitmask of seen help screens
func (s *State) SetHelpScreensSeen(seen uint32) error {
	s.HelpScreensSeen = seen
	return SaveState(s)
}
