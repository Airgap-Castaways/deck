package mcpfake

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/Airgap-Castaways/deck/internal/askconfig"
	"github.com/mark3labs/mcp-go/mcp"
)

type Request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      *mcp.RequestId  `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type Response struct {
	JSONRPC string         `json:"jsonrpc"`
	ID      *mcp.RequestId `json:"id,omitempty"`
	Result  any            `json:"result,omitempty"`
}

func Server(t interface{ Helper() }, name string, mode string) askconfig.MCPServer {
	t.Helper()
	return askconfig.MCPServer{Name: name, RunCommand: os.Args[0], Args: []string{"-test.run=TestMCPFakeProcess", "--", mode}}
}

func ModeFromArgs() string {
	for idx, arg := range os.Args {
		if arg == "--" && idx+1 < len(os.Args) {
			return strings.TrimSpace(os.Args[idx+1])
		}
	}
	return ""
}

func Serve(mode string, handler func(string, Request) (*Response, bool)) {
	reader := bufio.NewReader(os.Stdin)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			os.Exit(0)
		}
		line = strings.TrimRight(line, "\r\n")
		var req Request
		if err := json.Unmarshal([]byte(line), &req); err != nil {
			continue
		}
		resp, exitAfter := handler(mode, req)
		if resp != nil {
			raw, err := json.Marshal(resp)
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(2)
			}
			if _, err := fmt.Fprintf(os.Stdout, "%s\n", raw); err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(2)
			}
		}
		if exitAfter {
			os.Exit(0)
		}
	}
}

func ToolCall(req Request) (string, map[string]any) {
	params := struct {
		Name      string         `json:"name"`
		Arguments map[string]any `json:"arguments"`
	}{}
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return "", nil
	}
	return params.Name, params.Arguments
}

func StringArg(args map[string]any, key string) string {
	if args == nil {
		return ""
	}
	value, ok := args[key]
	if !ok {
		return ""
	}
	text, _ := value.(string)
	return strings.TrimSpace(text)
}
