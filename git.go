package aico

import (
	"bytes"
	"os"
	"os/exec"
)

// ExecuteGitDiff runs the `git diff` command and returns its output.
func ExecuteGitDiffStaged() (string, error) {
	cmd := exec.Command("git", "diff", "--staged")
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		return "", err
	}
	return out.String(), nil
}

// CommitChanges runs the `git commit` command with the selected commit message.
func CommitChanges(commitMessage string) error {
	cmd := exec.Command("git", "commit", "-m", commitMessage)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
