package testutils

import (
	"errors"
	"fmt"
	"os/exec"
)

func CheckDockerIsRunning() (err error) {
	cmd := exec.Command("docker", "info")
	if err = cmd.Run(); err != nil {
		err = errors.New("docker not running")
	}
	return
}

func PullDockerImage(dbImage string) error {
	cmd := exec.Command("docker", "pull", dbImage)

	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to pull %s: %s:\n%s", dbImage, err, output)
	}
	return nil
}
