package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/qbee-io/qbee-agent/app/inventory"
	"github.com/qbee-io/qbee-agent/app/log"
)

type BootstrapRequest struct {
	// Host - The name of the current host, according to the kernel.
	// It is undefined whether this is qualified or unqualified with a domain name.
	Host string `json:"host"`

	// FQHost - The fully qualified name of the host (e.g. "device1.example.com").
	FQHost string `json:"fqhost"`

	// UQHost - The unqualified name of the host (e.g. "device1").
	UQHost string `json:"uqhost"`

	// HardwareMAC - This contains the MAC address of the named interface map[interface]macAddress.
	// Note: The keys in this array are canonified.
	// For example, the entry for wlan0.1 would be found under the wlan0_1 key.
	//
	// Example:
	// {
	// 	"ens1": "52:54:00:4a:db:ee",
	//  "qbee0": "00:00:00:00:00:00"
	// }
	HardwareMAC map[string]string `json:"hardware_mac"`

	// IPDefault - All four octets of the IPv4 address of the first system interface.
	//Note: If the system has a single ethernet interface, this variable will contain the IPv4 address.
	// However, if the system has multiple interfaces, then this variable will simply be the IPv4 address of the first
	// interface in the list that has an assigned address.
	// Use IPv4[interface_name] for details on obtaining the IPv4 addresses of all interfaces on a system.
	IPDefault string `json:"ip_default"`

	// IPv4 - All IPv4 addresses of the system mapped by interface name.
	// Example:
	// {
	//	"ens1": "192.168.122.239",
	//	"qbee0": "100.64.39.78"
	// }
	IPv4 map[string]string `json:"ipv4"`

	// RawPublicKey of the device as slice of PEM-encoded lines.
	// Example:
	// []string{
	//    "-----BEGIN PUBLIC KEY-----",
	//    "MIGbMBAGByqGSM49AgEGBSuBBAAjA4GGAAQBvDALiaU+dyvd1DhMUCEXnuX4h5r5",
	//    "ikBVNSl88QBtBoxtQy1XsCJ7Dm/tzoQ1YPYT80oVTdExK/oFnZFvI89SX8sBN89L",
	//    "Y8q+8BBQPLf1nA3DG7apq7xq11Zkpde2eK0pCUG7nZPisXlU96C44NLE62TzDYEZ",
	//    "RYkhJQhFeNOlFSpF/xA=",
	//    "-----END PUBLIC KEY-----"
	// }
	RawPublicKey []string `json:"pub_key"`
}

type BootstrapResponse struct {
	Status                    string   `json:"status"`
	CertificateRequestsStatus string   `json:"cert_req"`
	Certificate               []string `json:"cert"`
	CACertificate             []string `json:"ca_cert"`
}

const boostrapWaitTime = 5 * time.Second

// Bootstrap device using agent's config and provided bootstrap key.
func Bootstrap(ctx context.Context, cfg *Config, bootstrapKey string) error {
	agent, err := New(cfg)
	if err != nil {
		return err
	}

	if err = agent.createCredentialsDirectory(); err != nil {
		return err
	}

	if err = agent.loadCACertificate(); err != nil {
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

		if response, err = agent.sendBootstrapRequest(ctx, bootstrapKey, bootstrapRequest); err != nil {
			return fmt.Errorf("error sending bootstrap request: %w", err)
		}

		if response.CertificateRequestsStatus == "authorized" {
			break
		}

		log.Infof("Awaiting to be approved.")
		time.Sleep(boostrapWaitTime)
	}

	pemCert := []byte(strings.Join(response.Certificate, "\n"))
	if err = agent.saveCertificate(pemCert); err != nil {
		return err
	}

	if err = agent.saveConfig(); err != nil {
		return err
	}

	if err = agent.sendSystemInventory(ctx); err != nil {
		return err
	}

	log.Infof("Bootstrap successfully completed")

	return nil
}

// newBootstrapRequest returns a new BootstrapRequest for the agent.
func (agent *Agent) newBootstrapRequest() (*BootstrapRequest, error) {
	log.Infof("Gathering system information")

	systemInfo, err := inventory.CollectSystemInfo()
	if err != nil {
		return nil, fmt.Errorf("error collecting system info: %w", err)
	}

	var rawPublicKey []string
	if rawPublicKey, err = agent.rawPublicKey(); err != nil {
		return nil, err
	}

	bootstrapRequest := &BootstrapRequest{
		Host:         systemInfo.Host,
		FQHost:       systemInfo.FQHost,
		UQHost:       systemInfo.UQHost,
		HardwareMAC:  systemInfo.HardwareMAC,
		IPDefault:    systemInfo.IPv4First,
		IPv4:         systemInfo.IPv4,
		RawPublicKey: rawPublicKey,
	}

	return bootstrapRequest, nil
}

// sendBootstrapRequest sends bootstrap request to the device hub.
func (agent *Agent) sendBootstrapRequest(
	ctx context.Context,
	bootstrapKey string,
	req *BootstrapRequest,
) (*BootstrapResponse, error) {

	endpoint := fmt.Sprintf(
		"https://%s:%s/v1/org/device/xauth/bootstrap",
		agent.cfg.DeviceHubServer, agent.cfg.DeviceHubPort)

	buf := new(bytes.Buffer)
	if err := json.NewEncoder(buf).Encode(req); err != nil {
		return nil, fmt.Errorf("error marshaling bootstrap request body: %w", err)
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, buf)
	if err != nil {
		return nil, fmt.Errorf("error preparing bootstrap request: %w", err)
	}

	request.Header.Set("Authorization", fmt.Sprintf("token %s", bootstrapKey))

	var response *http.Response
	if response, err = agent.anonymousHTTPClient().Do(request); err != nil {
		return nil, fmt.Errorf("error sending bootstrap request: %w", err)
	}

	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		responseBody, _ := io.ReadAll(response.Body)
		return nil, fmt.Errorf("unexpected bootstrap response status: %d %s", response.StatusCode, responseBody)
	}

	bootstrapResponse := new(BootstrapResponse)

	if err = json.NewDecoder(response.Body).Decode(bootstrapResponse); err != nil {
		return nil, fmt.Errorf("error decoding bootstrap repsonse body: %w", err)
	}

	return bootstrapResponse, nil
}
