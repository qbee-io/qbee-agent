package remoteaccess

import (
	"context"
	"fmt"

	"github.com/qbee-io/qbee-agent/app/binary"
)

type API interface {
	binary.API
	Get(ctx context.Context, path string, dst any) error
}

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
