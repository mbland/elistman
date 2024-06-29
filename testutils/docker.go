package testutils

import (
	"errors"
	"fmt"
	"log"
	"os/exec"
	"strings"
)

func CheckDockerIsRunning() (err error) {
	cmd := exec.Command("docker", "info")
	if err = cmd.Run(); err != nil {
		err = errors.New("docker must be running to run this test")
	}
	return
}

func PullDockerImage(dbImage string) error {
	cmd := exec.Command("docker", "pull", dbImage)

	if err := CheckDockerIsRunning(); err != nil {
		return err
	} else if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to pull %s: %s:\n%s", dbImage, err, output)
	}
	return nil
}

func LaunchDockerContainer(
	service string, localEndpoint BaseEndpoint, containerPort int, dbImage string,
) (cleanup func() error, err error) {
	portMap := fmt.Sprintf("%s:%d", localEndpoint, containerPort)
	cmd := exec.Command("docker", "run", "-d", "-p", portMap, dbImage)
	var output []byte

	if err = PullDockerImage(dbImage); err != nil {
		return
	} else if output, err = cmd.CombinedOutput(); err != nil {
		const errFmt = "failed to start local %s at %s: %s:\n%s"
		err = fmt.Errorf(errFmt, service, localEndpoint, err, output)
		return
	}

	const logFmt = "local %s running at %s with container ID: %s"
	containerId := strings.TrimSpace(string(output))
	log.Printf(logFmt, service, localEndpoint, containerId)

	cleanup = func() error {
		return CleanupDockerContainer(service, containerId)
	}
	return
}

func CleanupDockerContainer(service, containerId string) (errResult error) {
	stopCmd := exec.Command("docker", "stop", "-t", "0", containerId)
	rmCmd := exec.Command("docker", "rm", containerId)

	raiseErr := func(action string, err error, output []byte) error {
		const errFmt = "failed to %s %s container with ID %s: %s:\n%s"
		return fmt.Errorf(errFmt, action, service, containerId, err, output)
	}

	log.Printf("cleaning up %s container with ID: %s", service, containerId)

	if output, err := stopCmd.CombinedOutput(); err != nil {
		errResult = raiseErr("stop", err, output)
	} else if output, err = rmCmd.CombinedOutput(); err != nil {
		errResult = raiseErr("remove", err, output)
	}
	return
}
