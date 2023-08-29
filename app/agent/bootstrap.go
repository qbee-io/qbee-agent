package agent

import (
	"context"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/qbee-io/qbee-agent/app/inventory"
	"github.com/qbee-io/qbee-agent/app/log"
)

const bootstrapWaitTime = 5 * time.Second

// Bootstrap device using agent's config and provided bootstrap key.
func Bootstrap(ctx context.Context, cfg *Config) error {
	agent, err := NewWithoutCredentials(cfg)
	if err != nil {
		return err
	}

	if err = agent.createPrivateKey(); err != nil {
		return err
	}

	var bootstrapRequest *BootstrapRequest
	bootstrapRequest, err = agent.newBootstrapRequest()
	if err != nil {
		return err
	}

	var response *BootstrapResponse

	log.Infof("Sending bootstrap request to %s:%s", agent.cfg.DeviceHubServer, agent.cfg.DeviceHubPort)

	for {

		if response, err = agent.sendBootstrapRequest(ctx, cfg.BootstrapKey, bootstrapRequest); err != nil {
			return fmt.Errorf("error sending bootstrap request: %w", err)
		}

		if response.CertificateRequestsStatus == "authorized" {
			break
		}

		log.Infof("Awaiting to be approved.")
		time.Sleep(bootstrapWaitTime)
	}

	pemCert := []byte(strings.Join(response.Certificate, "\n"))

	if err = agent.saveCertificate(pemCert); err != nil {
		return err
	}

	// Do not record the bootstrap key in the config file
	cfg.BootstrapKey = ""

	if err = agent.saveConfig(); err != nil {
		return err
	}

	// re-initialize agent to make use of new credentials
	agent, err = New(cfg)
	if err != nil {
		return err
	}

	agent.RunOnce(ctx, QuickRun)
	agent.inProgress.Wait()

	log.Infof("Bootstrap successfully completed")

	upstartCmd := guessUpstartCommand("qbee-agent", "start")
	log.Infof("Please remember to start the qbee-agent service as administrative user")
	log.Infof("Detected start command based on OS attributes is: $ %s", upstartCmd)

	return nil
}

// guesssUpstartCommand guesses the upstart system based on available binaries
func guessUpstartCommand(progName, command string) string {
	// up%s is only used on linux
	if runtime.GOOS != "linux" {
		return "unknown"
	}
	// first check for systemd
	if _, err := exec.LookPath("systemctl"); err == nil {
		return fmt.Sprintf("systemctl %s %s", command, progName)
	}
	// then check for sysvinit
	if _, err := exec.LookPath("service"); err == nil {
		return fmt.Sprintf("service %s %s", progName, command)
	}
	// then check for openrc
	if _, err := exec.LookPath("rc-service"); err == nil {
		return fmt.Sprintf("rc-service %s %s", progName, command)
	}
	// then check for upstart
	if _, err := exec.LookPath("initctl"); err == nil {
		return fmt.Sprintf("initctl %s %s", command, progName)
	}
	// then check for runit
	if _, err := exec.LookPath("sv"); err == nil {
		return fmt.Sprintf("sv %s %s", command, progName)
	}
	// then check for launchctl
	if _, err := exec.LookPath("launchctl"); err == nil {
		return fmt.Sprintf("launchctl %s %s", command, progName)
	}
	// then check for rcctl
	if _, err := exec.LookPath("rcctl"); err == nil {
		return fmt.Sprintf("rcctl %s %s", command, progName)
	}
	// then check existence of /etc/init.d/qbee-agent
	if _, err := exec.LookPath(fmt.Sprintf("/etc/init.d/%s", progName)); err == nil {
		return fmt.Sprintf("/etc/init.d/%s %s", progName, command)
	}

	return "unknown"
}

// getRawPublicKey returns a slice of PEM-encoded public key lines.
func (agent *Agent) getRawPublicKey() ([]string, error) {
	if agent.privateKey == nil {
		return nil, fmt.Errorf("agent does not have a private key set")
	}

	publicKey := &agent.privateKey.PublicKey

	publicKeyDER, err := x509.MarshalPKIXPublicKey(publicKey)
	if err != nil {
		return nil, fmt.Errorf("error marshaling public key: %w", err)
	}

	pemBlock := pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: publicKeyDER,
	}

	return strings.Split(string(pem.EncodeToMemory(&pemBlock)), "\n"), nil
}

// newBootstrapRequest returns a new BootstrapRequest for the agent.
func (agent *Agent) newBootstrapRequest() (*BootstrapRequest, error) {
	log.Infof("Gathering system information")

	systemInventory, err := inventory.CollectSystemInventory()
	if err != nil {
		return nil, fmt.Errorf("error collecting system info: %w", err)
	}

	var rawPublicKey []string
	if rawPublicKey, err = agent.getRawPublicKey(); err != nil {
		return nil, err
	}

	systemInfo := systemInventory.System

	bootstrapRequest := &BootstrapRequest{
		Host:         systemInfo.Host,
		FQHost:       systemInfo.FQHost,
		UQHost:       systemInfo.UQHost,
		HardwareMAC:  systemInfo.HardwareMAC,
		IPDefault:    systemInfo.IPv4First,
		IPv4:         systemInfo.IPv4,
		RawPublicKey: rawPublicKey,
	}

	if agent.cfg.DeviceName != "" {
		bootstrapRequest.DeviceName = agent.cfg.DeviceName
	}

	return bootstrapRequest, nil
}
