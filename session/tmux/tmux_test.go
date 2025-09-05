package tmux

import (
	cmd2 "claude-squad/cmd"
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"claude-squad/cmd/cmd_test"

	"github.com/stretchr/testify/require"
)

type MockPtyFactory struct {
	t *testing.T

	// Array of commands and the corresponding file handles representing PTYs.
	cmds  []*exec.Cmd
	files []*os.File
}

func (pt *MockPtyFactory) Start(cmd *exec.Cmd) (*os.File, error) {
	filePath := filepath.Join(pt.t.TempDir(), fmt.Sprintf("pty-%s-%d", pt.t.Name(), rand.Int31()))
	f, err := os.OpenFile(filePath, os.O_CREATE|os.O_RDWR, 0644)
	if err == nil {
		pt.cmds = append(pt.cmds, cmd)
		pt.files = append(pt.files, f)
	}
	return f, err
}

func (pt *MockPtyFactory) Close() {}

func NewMockPtyFactory(t *testing.T) *MockPtyFactory {
	return &MockPtyFactory{
		t: t,
	}
}

func TestSanitizeName(t *testing.T) {
	session := NewTmuxSession("asdf", "program")
	require.Equal(t, TmuxPrefix+"asdf", session.sanitizedName)

	session = NewTmuxSession("a sd f . . asdf", "program")
	require.Equal(t, TmuxPrefix+"asdf__asdf", session.sanitizedName)
}

func TestStartTmuxSession(t *testing.T) {
	ptyFactory := NewMockPtyFactory(t)

	created := false
	cmdExec := cmd_test.MockCmdExec{
		RunFunc: func(cmd *exec.Cmd) error {
			if strings.Contains(cmd.String(), "has-session") && !created {
				created = true
				return fmt.Errorf("session already exists")
			}
			return nil
		},
		OutputFunc: func(cmd *exec.Cmd) ([]byte, error) {
			return []byte("output"), nil
		},
	}

	workdir := t.TempDir()
	session := newTmuxSession("test-session", "claude", ptyFactory, cmdExec)

	err := session.Start(workdir)
	require.NoError(t, err)
	require.Equal(t, 2, len(ptyFactory.cmds))
	require.Equal(t, fmt.Sprintf("tmux new-session -d -s claudesquad_test-session -c %s claude", workdir),
		cmd2.ToString(ptyFactory.cmds[0]))
	require.Equal(t, "tmux attach-session -t claudesquad_test-session",
		cmd2.ToString(ptyFactory.cmds[1]))

	require.Equal(t, 2, len(ptyFactory.files))

	// File should be closed.
	_, err = ptyFactory.files[0].Stat()
	require.Error(t, err)
	// File should be open
	_, err = ptyFactory.files[1].Stat()
	require.NoError(t, err)
}

func TestEnvironmentVariablePropagation(t *testing.T) {
	ptyFactory := NewMockPtyFactory(t)

	created := false
	cmdExec := cmd_test.MockCmdExec{
		RunFunc: func(cmd *exec.Cmd) error {
			if strings.Contains(cmd.String(), "has-session") && !created {
				created = true
				return fmt.Errorf("session already exists")
			}
			return nil
		},
		OutputFunc: func(cmd *exec.Cmd) ([]byte, error) {
			return []byte("output"), nil
		},
	}

	// Set a test environment variable
	testEnvVar := "CLAUDE_SQUAD_TEST_ENV_VAR"
	testEnvValue := "test_value_12345"
	os.Setenv(testEnvVar, testEnvValue)
	defer os.Unsetenv(testEnvVar)

	workdir := t.TempDir()
	session := newTmuxSession("env-test-session", "claude", ptyFactory, cmdExec)

	err := session.Start(workdir)
	require.NoError(t, err)
	require.Equal(t, 2, len(ptyFactory.cmds))

	// Check that the new-session command has environment variables
	newSessionCmd := ptyFactory.cmds[0]
	require.NotNil(t, newSessionCmd.Env, "Environment should be set for new-session command")

	// Check that our test environment variable is present
	foundEnvVar := false
	for _, env := range newSessionCmd.Env {
		if strings.HasPrefix(env, testEnvVar+"=") {
			expectedEnv := testEnvVar + "=" + testEnvValue
			require.Equal(t, expectedEnv, env, "Test environment variable should be correctly propagated")
			foundEnvVar = true
			break
		}
	}
	require.True(t, foundEnvVar, "Test environment variable should be present in command environment")

	// Check that the attach-session command also has environment variables
	attachCmd := ptyFactory.cmds[1]
	require.NotNil(t, attachCmd.Env, "Environment should be set for attach-session command")
}
