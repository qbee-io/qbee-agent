//go:build windows

package agent

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"go.qbee.io/agent/app/log"
	"golang.org/x/sys/windows/svc"
)

type myService struct{}

func (agent *Agent) Execute(args []string, r <-chan svc.ChangeRequest, status chan<- svc.Status) (bool, uint32) {

	ctx := context.Background()
	intervalChange := agent.Configuration.RunIntervalChangedNotifier()

	const cmdsAccepted = svc.AcceptStop | svc.AcceptShutdown | svc.AcceptPauseAndContinue

	status <- svc.Status{State: svc.StartPending}

	status <- svc.Status{State: svc.Running, Accepts: cmdsAccepted}

	stopSignalCh := make(chan os.Signal, 1)
	signal.Notify(stopSignalCh, os.Interrupt, syscall.SIGTERM)

loop:
	for {
		select {
		case <-agent.reboot:
			agent.RebootSystem(ctx)
		case <-stopSignalCh:
			log.Debugf("received interrupt signal")

			agent.stop <- true

		case newInterval := <-intervalChange:
			log.Debugf("run interval updated: %s", newInterval)
			agent.loopTicker.Reset(newInterval)

		case <-agent.loopTicker.C:
			go agent.RunOnce(ctx, FullRun)

		case <-agent.update:
			// reset the ticker, so we don't run the update twice (scheduled and manually triggered)
			agent.loopTicker.Reset(agent.Configuration.RunInterval())

			go agent.RunOnce(ctx, FullRun)

		case c := <-r:
			switch c.Cmd {
			case svc.Interrogate:
				status <- c.CurrentStatus
			case svc.Stop, svc.Shutdown:
				log.Infof("stopping the agent")

				// stop the remote access service (if running)
				if err := agent.remoteAccess.Stop(); err != nil {
					log.Errorf("failed to stop remote access: %s", err)
				}

				// let all the processing finish
				agent.Wait()

				break loop
			case svc.Pause:
				status <- svc.Status{State: svc.Paused, Accepts: cmdsAccepted}
			case svc.Continue:
				status <- svc.Status{State: svc.Running, Accepts: cmdsAccepted}
			default:
				log.Errorf("Unexpected service control request #%d", c)
			}
		}
	}

	status <- svc.Status{State: svc.StopPending}
	return false, 1
}

func RunService(cfg *Config) error {
	agent, err := New(cfg)
	if err != nil {
		return fmt.Errorf("error initializing the agent: %w", err)
	}

	return svc.Run("qbee-agent", agent)

}
