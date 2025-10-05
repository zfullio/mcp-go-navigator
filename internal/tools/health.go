package tools

import (
	"errors"
	"os/exec"
)

func HealthCheck() error {
	_, err := exec.LookPath("go")
	if err != nil {
		return errors.New("go compiler not found in PATH")
	}

	return nil
}
