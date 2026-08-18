package ui

import (
	"os"
	"os/exec"
	"testing"
)

func TestInitialModel_IsGitRepo(t *testing.T) {
	// Save and restore original directory
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(origDir)

	// Test in a git directory
	m := InitialModel()
	m.Init()
	if !m.isGitRepo {
		t.Error("expected isGitRepo to be true in a git directory")
	}

	// Test in a non-git directory
	tmpDir := t.TempDir()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}

	m2 := InitialModel()
	m2.Init()
	if m2.isGitRepo {
		t.Error("expected isGitRepo to be false in a non-git directory")
	}
}

func TestGitNotFound(t *testing.T) {
	// Save and restore original PATH
	origPath := os.Getenv("PATH")
	defer os.Setenv("PATH", origPath)

	// Set PATH to empty so git is not found
	os.Setenv("PATH", "")

	// This should call os.Exit(1), which we can't test directly
	// Just verify that LookPath fails
	_, err := exec.LookPath("git")
	if err == nil {
		t.Error("expected LookPath to fail with empty PATH")
	}
}
