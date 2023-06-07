package remoteaccess

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/qbee-io/qbee-agent/app/api"
	"github.com/qbee-io/qbee-agent/app/binary"
	"github.com/qbee-io/qbee-agent/app/log"
	"github.com/qbee-io/qbee-agent/app/utils"
)

const NetworkInterfaceName = "qbee0"

// New creates a new instance of the remote access service.
func New(apiClient *api.Client, server, certDir, binDir string, proxy *api.Proxy) *Service {
	return &Service{
		server:       server,
		api:          apiClient,
		binDir:       binDir,
		certDir:      certDir,
		proxy:        proxy,
		stopLoop:     make(chan bool, 1),
		notification: make(chan bool, 1),
	}
}

// Service controls remote access for the agent.
type Service struct {
	enabled      bool
	server       string
	binDir       string
	certDir      string
	api          *api.Client
	proxy        *api.Proxy
	cmd          *exec.Cmd
	lock         sync.Mutex
	credentials  Credentials
	loopRunning  bool
	stopLoop     chan bool
	notification chan bool

	// activeProcesses is a wait group that keeps track of all active processes.
	// It's used for testing only.
	activeProcesses sync.WaitGroup
}

// GetNotificationChannel returns a channel that is used to notify user about remote access state changes.
func (s *Service) GetNotificationChannel() <-chan bool {
	return s.notification
}

// Stop disables remote access.
func (s *Service) Stop() error {
	return s.disable()
}

// binPath returns the path to the openvpn binary.
func (s *Service) binPath() string {
	return filepath.Join(s.binDir, binary.OpenVPN)
}

// caCertPath returns the path to the VPN CA certificate.
func (s *Service) caCertPath() string {
	return filepath.Join(s.certDir, "qbee-ca-vpn.cert")
}

// certPath returns the path to the VPN certificate.
func (s *Service) certPath() string {
	return filepath.Join(s.certDir, "qbee-vpn.cert")
}

// keyPath returns the path to the VPN private key.
func (s *Service) keyPath() string {
	return filepath.Join(s.certDir, "qbee.key")
}

// UpdateState ensures that remote access is enabled or disabled based on the enabled parameter.
func (s *Service) UpdateState(ctx context.Context, expectedActive bool) error {
	if !s.lock.TryLock() {
		return nil
	}
	defer s.lock.Unlock()

	s.enabled = expectedActive

	isActive := s.checkStatus()

	if !expectedActive && isActive {
		return s.disable()
	}

	if !isActive {
		if err := s.enable(ctx); err != nil {
			return err
		}
	}

	if !s.loopRunning {
		go s.startWatchdogLoop()
	}

	return nil
}

const loopInterval = time.Minute

// startWatchdogLoop for the remote access service.
func (s *Service) startWatchdogLoop() {
	ticker := time.NewTicker(loopInterval)

	defer func() {
		ticker.Stop()

		s.lock.Lock()
		s.loopRunning = false
		s.lock.Unlock()

		if err := recover(); err != nil {
			log.Errorf("remote access watchdog loop crashed:", err)
		}

		s.notification <- true
	}()

	for {
		select {
		case <-s.stopLoop:
			return
		case <-ticker.C:
			s.ensureRunning()
		}
	}
}

// checkStatus of the remote access service and return true if enabled.
func (s *Service) checkStatus() bool {
	if s.cmd == nil {
		return false
	}

	return s.cmd.Process != nil && s.cmd.ProcessState == nil
}

// enable remote access.
func (s *Service) enable(ctx context.Context) error {
	if err := s.loadTUNModule(ctx); err != nil {
		return err
	}

	if err := s.downloadOpenVPN(ctx); err != nil {
		return err
	}

	if err := s.refreshCredentials(ctx); err != nil {
		return err
	}

	return s.start()
}

func (s *Service) start() error {
	args := []string{
		"--client",
		"--remote", s.server,
		"--comp-lzo",
		"--dev", NetworkInterfaceName,
		"--dev-type", "tun",
		"--proto", "tcp",
		"--port", "443",
		"--nobind",
		"--auth-nocache",
		"--script-security", "1",
		"--persist-key",
		"--persist-tun",
		"--ca", s.caCertPath(),
		"--cert", s.certPath(),
		"--key", s.keyPath(),
		"--verb", "0",
		"--suppress-timestamps",
		"--remote-cert-tls", "server",
		"--disable-occ",
		"--cipher", "AES-256-GCM",
	}

	// add proxy settings if configured
	if s.proxy != nil {
		args = append(args, "--http-proxy", s.proxy.Host, s.proxy.Port)

		if s.proxy.User == "" {
			proxyAuthFile := filepath.Join(s.certDir, "qbee-vpn-password")
			proxyAuthFileContents := fmt.Sprintf("%s\n%s", s.proxy.User, s.proxy.Password)

			if err := os.WriteFile(proxyAuthFile, []byte(proxyAuthFileContents), 0600); err != nil {
				return fmt.Errorf("failed to write proxy auth file: %w", err)
			}

			args = append(args, proxyAuthFile, "basic")
		}
	}

	s.cmd = exec.Command(s.binPath(), args...)
	s.cmd.Stdout = log.NewWriter(log.DEBUG, "remote-access: ")
	s.cmd.Stderr = log.NewWriter(log.DEBUG, "remote-access: ")
	s.cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid:   true,
		Pdeathsig: syscall.SIGKILL,
	}

	if err := s.cmd.Start(); err != nil {
		return fmt.Errorf("failed to start remote access process: %w", err)
	}

	s.activeProcesses.Add(1)

	go func() {
		go s.notifyWhenInterfaceReady()

		if err := s.cmd.Wait(); err != nil {
			log.Errorf("remote access process exited with error: %v", err)
		}

		s.activeProcesses.Done()
	}()

	return nil
}

func (s *Service) notifyWhenInterfaceReady() {
	for i := 0; i < 10; i++ {
		if !s.loopRunning {
			return
		}

		interfaces, err := net.Interfaces()
		if err != nil {
			log.Errorf("failed to list network interfaces: %v", err)
			return
		}

		for _, networkInterface := range interfaces {
			if networkInterface.Name == NetworkInterfaceName {
				s.notification <- true
			}
		}

		time.Sleep(time.Second)
	}
}

func (s *Service) stop() error {
	if s.cmd == nil || s.cmd.Process == nil || s.cmd.ProcessState != nil {
		return fmt.Errorf("cannot stop remote access - already not running")
	}

	if err := syscall.Kill(-s.cmd.Process.Pid, syscall.SIGINT); err != nil {
		return fmt.Errorf("failed to kill remote access process: %w", err)
	}
	return nil
}

// disable remote access.
func (s *Service) disable() error {
	if err := s.stop(); err != nil {
		return err
	}

	s.stopLoop <- true

	return nil
}

const refreshBeforeExpiry = 15 * time.Minute

const secretFileMode = 0600

// refreshCredentials when missing or soon expiring.
func (s *Service) refreshCredentials(ctx context.Context) error {
	if s.credentials.Expiry > time.Now().Add(refreshBeforeExpiry).Unix() {
		return nil
	}

	credentials, err := s.getCredentials(ctx)
	if err != nil {
		return err
	}

	if err = os.WriteFile(s.caCertPath(), credentials.CACertificatePEM(), secretFileMode); err != nil {
		return fmt.Errorf("failed to write CA certificate: %w", err)
	}

	if err = os.WriteFile(s.certPath(), credentials.CertificatePEM(), secretFileMode); err != nil {
		return fmt.Errorf("failed to write certificate: %w", err)
	}

	s.credentials = *credentials

	// return nil if we do not have any process running
	if !s.checkStatus() {
		return nil
	}

	// restart process if we have one running
	if err = s.stop(); err != nil {
		return err
	}

	return nil
}

// loadTUNModule attempts to load TUN module if /dev/net/tun doesn't exist.
// Returns true if /dev/net/tun is available.
func (s *Service) loadTUNModule(ctx context.Context) error {
	if _, err := os.Stat("/dev/net/tun"); err == nil {
		return nil
	}

	modprobe, err := exec.LookPath("modprobe")
	if err != nil {
		return fmt.Errorf("modprobe not found: %w", err)
	}

	if _, err = utils.RunCommand(ctx, []string{modprobe, "tun"}); err != nil {
		return fmt.Errorf("failed to load TUN module: %w", err)
	}

	if _, err := os.Stat("/dev/net/tun"); err != nil {
		return fmt.Errorf("/dev/net/tun error: %w", err)
	}

	return nil
}

// downloadOpenVPN binary.
func (s *Service) downloadOpenVPN(ctx context.Context) error {
	binPath := s.binPath()

	// check if binary already exists
	if _, err := os.Stat(binPath); err == nil {
		return nil
	}

	// download the binary if not found
	if err := binary.Download(s.api, ctx, binary.OpenVPN, binPath); err != nil {
		return err
	}

	return nil
}

// ensureRunning checks if remote access is running and restarts it if not.
func (s *Service) ensureRunning() {
	s.lock.Lock()
	defer s.lock.Unlock()

	if !s.enabled {
		return
	}

	if err := s.refreshCredentials(context.Background()); err != nil {
		log.Errorf("failed to refresh remote access credentials:", err)
	}

	if !s.checkStatus() {
		if err := s.start(); err != nil {
			log.Errorf("failed to restart remote access:", err)
		}
	}
}
