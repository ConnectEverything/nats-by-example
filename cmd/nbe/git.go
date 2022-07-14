package main

import (
	"os"
	"os/exec"
)

func cloneRepo(repo, dir string) error {
	cmd := exec.Command("git", "clone", repo, "--single-branch", dir)
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	return cmd.Run()
}

func updateRepo(repo, dir string) error {
	cmd := exec.Command("git", "-C", dir, "fetch", repo)
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	return cmd.Run()
}
