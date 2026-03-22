package install

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/taedi90/deck/internal/stepspec"
	"github.com/taedi90/deck/internal/workflowexec"
)

func runVerifyImages(ctx context.Context, spec map[string]any) error {
	decoded, err := workflowexec.DecodeSpec[stepspec.VerifyImage](spec)
	if err != nil {
		return fmt.Errorf("decode VerifyImage spec: %w", err)
	}
	required := decoded.Images
	if len(required) == 0 {
		return fmt.Errorf("%s: VerifyImages requires images", errCodeInstallImagesMissing)
	}

	cmdArgs := decoded.Command
	if len(cmdArgs) == 0 {
		cmdArgs = []string{"ctr", "-n", "k8s.io", "images", "list", "-q"}
	}

	timeout := parseStepTimeout(decoded.Timeout, 20*time.Second)

	output, err := runCommandOutputWithContext(ctx, cmdArgs, timeout)
	if err != nil {
		if errors.Is(err, ErrStepCommandTimeout) || errors.Is(err, context.DeadlineExceeded) {
			return fmt.Errorf("%s: image verification timed out: %w", errCodeInstallImagesCmdFailed, err)
		}
		return fmt.Errorf("%s: %w", errCodeInstallImagesCmdFailed, err)
	}

	available := map[string]bool{}
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		available[line] = true
	}

	missing := make([]string, 0)
	for _, image := range required {
		if !available[image] {
			missing = append(missing, image)
		}
	}

	if len(missing) > 0 {
		return fmt.Errorf("%s: missing images: %s", errCodeInstallImagesNotFound, strings.Join(missing, ", "))
	}

	return nil
}
