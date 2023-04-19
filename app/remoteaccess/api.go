package remoteaccess

import (
	"context"
	"fmt"
)

const vpnCertPath = "/v1/org/device/auth/vpncert"

// getCredentials retrieves the remote access credentials from the device hub.
func (s *Service) getCredentials(ctx context.Context) (*Credentials, error) {
	credentials := new(Credentials)

	if err := s.api.Get(ctx, vpnCertPath, &credentials); err != nil {
		return nil, err
	}

	if credentials.Status != "OK" {
		return nil, fmt.Errorf("failed to get remote access credentials")
	}

	return credentials, nil
}
