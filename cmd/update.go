package cmd

// update self-updates the meta-ads-cli binary by cloning the latest source from GitHub,
// rebuilding it, and atomically replacing the current executable.
//
// Requires: git, go

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/spf13/cobra"
)

const repoURL = "https://github.com/the20100/meta-ads-cli"

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update meta-ads-cli to the latest version from GitHub",
	Long: `Pull the latest source from GitHub, rebuild, and replace the current binary.

Requires git and go to be installed (same dependencies as the initial install).`,
	RunE: runUpdate,
}

func init() {
	rootCmd.AddCommand(updateCmd)
}

func runUpdate(cmd *cobra.Command, args []string) error {
	out := cmd.OutOrStdout()

	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("finding current binary: %w", err)
	}
	exe, err = filepath.EvalSymlinks(exe)
	if err != nil {
		return fmt.Errorf("resolving binary path: %w", err)
	}

	fmt.Fprintf(out, "Updating binary at %s\n\n", exe)

	tmpDir, err := os.MkdirTemp("", "meta-ads-cli-update-*")
	if err != nil {
		return fmt.Errorf("creating temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	fmt.Fprintln(out, "→ Cloning latest source...")
	if err := streamCmd(cmd, tmpDir, "git", "clone", "--depth=1", repoURL, "."); err != nil {
		return fmt.Errorf("git clone failed: %w", err)
	}

	fmt.Fprintln(out, "→ Building...")
	newBin := filepath.Join(tmpDir, "meta-ads-cli")
	if err := streamCmd(cmd, tmpDir, "go", "build", "-o", newBin, "."); err != nil {
		return fmt.Errorf("go build failed: %w", err)
	}

	fmt.Fprintln(out, "→ Installing...")
	if err := atomicReplace(newBin, exe); err != nil {
		return fmt.Errorf("replacing binary: %w", err)
	}

	fmt.Fprintln(out, "\n✓ Updated successfully.")
	return nil
}

func streamCmd(cmd *cobra.Command, dir, name string, args ...string) error {
	c := exec.Command(name, args...)
	c.Dir = dir
	c.Stdout = cmd.OutOrStdout()
	c.Stderr = cmd.ErrOrStderr()
	return c.Run()
}

func atomicReplace(src, dst string) error {
	dstInfo, err := os.Stat(dst)
	if err != nil {
		return fmt.Errorf("stat destination: %w", err)
	}

	dstDir := filepath.Dir(dst)
	tmp, err := os.CreateTemp(dstDir, ".update-*")
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)

	srcFile, err := os.Open(src)
	if err != nil {
		tmp.Close()
		return fmt.Errorf("opening source binary: %w", err)
	}
	defer srcFile.Close()

	if _, err := io.Copy(tmp, srcFile); err != nil {
		tmp.Close()
		return fmt.Errorf("copying binary: %w", err)
	}
	tmp.Close()

	if err := os.Chmod(tmpName, dstInfo.Mode()); err != nil {
		return fmt.Errorf("setting permissions: %w", err)
	}

	if err := os.Rename(tmpName, dst); err != nil {
		return fmt.Errorf("renaming binary: %w", err)
	}

	return nil
}
