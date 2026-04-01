package askcli

import (
	"bufio"
	"fmt"
	"io"
	"strings"
	"syscall"

	"github.com/Airgap-Castaways/deck/internal/askcontract"
)

var interactiveSessionProbe = isInteractiveSession

type fileDescriptor interface{ Fd() uintptr }

func isInteractiveSession(stdin io.Reader, stdout io.Writer) bool {
	inFD, ok := stdin.(fileDescriptor)
	_ = stdout
	if !ok || !isCharDevice(inFD.Fd(), "stdin") {
		return false
	}
	return true
}

func isCharDevice(fd uintptr, name string) bool {
	_ = name
	var stat syscall.Stat_t
	if err := syscall.Fstat(int(fd), &stat); err != nil {
		return false
	}
	return stat.Mode&syscall.S_IFMT == syscall.S_IFCHR
}

func runInteractiveClarifications(stdin io.Reader, stdout io.Writer, plan askcontract.PlanResponse) (askcontract.PlanResponse, bool, error) {
	reader := bufio.NewReader(stdin)
	current := plan
	for {
		items := blockingClarifications(current)
		if len(items) == 0 {
			return current, false, nil
		}
		for _, item := range items {
			answer, aborted, err := promptClarification(reader, stdout, item)
			if err != nil {
				return current, false, err
			}
			if aborted {
				return current, true, nil
			}
			updated, err := applyPlanAnswers(current, []string{item.ID + "=" + answer})
			if err != nil {
				return current, false, err
			}
			current = updated
		}
	}
}

func blockingClarifications(plan askcontract.PlanResponse) []askcontract.PlanClarification {
	items := []askcontract.PlanClarification{}
	for _, item := range plan.Clarifications {
		if item.BlocksGeneration && strings.TrimSpace(item.Answer) == "" {
			items = append(items, item)
		}
	}
	return items
}

func promptClarification(reader *bufio.Reader, stdout io.Writer, item askcontract.PlanClarification) (string, bool, error) {
	if _, err := fmt.Fprintf(stdout, "clarify: %s\n", strings.TrimSpace(item.Question)); err != nil {
		return "", false, err
	}
	flushOutput(stdout)
	if len(item.Options) > 0 {
		for i, option := range item.Options {
			if _, err := fmt.Fprintf(stdout, "%d) %s\n", i+1, strings.TrimSpace(option)); err != nil {
				return "", false, err
			}
		}
		flushOutput(stdout)
	}
	prompt := "answer"
	if strings.TrimSpace(item.RecommendedDefault) != "" {
		prompt = prompt + " [default: " + strings.TrimSpace(item.RecommendedDefault) + "]"
	}
	prompt += ": "
	if _, err := io.WriteString(stdout, prompt); err != nil {
		return "", false, err
	}
	flushOutput(stdout)
	input, err := reader.ReadString('\n')
	if err != nil && err != io.EOF {
		return "", false, err
	}
	input = strings.TrimSpace(input)
	switch strings.ToLower(input) {
	case "q", "quit", "exit":
		return "", true, nil
	}
	if input == "" && strings.TrimSpace(item.RecommendedDefault) != "" {
		return strings.TrimSpace(item.RecommendedDefault), false, nil
	}
	if len(item.Options) > 0 {
		for i, option := range item.Options {
			if input == fmt.Sprintf("%d", i+1) {
				return strings.TrimSpace(option), false, nil
			}
			if strings.EqualFold(strings.TrimSpace(option), input) {
				return strings.TrimSpace(option), false, nil
			}
		}
	}
	if strings.TrimSpace(input) == "" {
		return "", false, fmt.Errorf("clarification %q requires a value", item.ID)
	}
	return input, false, nil
}
