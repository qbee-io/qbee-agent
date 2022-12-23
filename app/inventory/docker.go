package inventory

import "os/exec"

// HasDocker returns true if host OS has docker installed.
func HasDocker() bool {
	if _, err := exec.LookPath("docker"); err != nil {
		return false
	}

	return true
}
