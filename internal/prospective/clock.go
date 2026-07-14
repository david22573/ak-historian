package prospective

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

type ClockChecker interface {
	Check(context.Context) (ClockEvidence, error)
}

type SystemClockChecker struct{}

func (SystemClockChecker) Check(ctx context.Context) (ClockEvidence, error) {
	checked := time.Now().UTC()
	commandCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	output, err := exec.CommandContext(commandCtx, "timedatectl", "show", "--property=NTPSynchronized", "--value").CombinedOutput()
	diagnostic := strings.TrimSpace(string(output))
	evidence := ClockEvidence{
		CheckedAtUTC: checked,
		Method:       "timedatectl show --property=NTPSynchronized --value",
		Synchronized: err == nil && strings.EqualFold(diagnostic, "yes"),
		Diagnostic:   diagnostic,
	}
	evidence.EvidenceHash = HashBytes([]byte(fmt.Sprintf("%s\n%s\n%t\n%s", evidence.CheckedAtUTC.Format(time.RFC3339Nano), evidence.Method, evidence.Synchronized, diagnostic)))
	if err != nil {
		return evidence, fmt.Errorf("clock synchronization evidence command failed: %w", err)
	}
	if !evidence.Synchronized {
		return evidence, fmt.Errorf("local clock is not NTP synchronized")
	}
	return evidence, nil
}

type StaticClockChecker struct {
	Evidence ClockEvidence
	Err      error
}

func (checker StaticClockChecker) Check(context.Context) (ClockEvidence, error) {
	return checker.Evidence, checker.Err
}
