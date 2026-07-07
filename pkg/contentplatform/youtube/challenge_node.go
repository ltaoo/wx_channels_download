package youtube

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

//go:embed jsc/solver/yt.solver.lib.min.js
var nodeSolverLib string

//go:embed jsc/solver/yt.solver.core.js
var nodeSolverCore string

func solvePlayerChallengesWithNode(ctx context.Context, playerJS string, challengeType string, challenges []string) (map[string]string, error) {
	if len(challenges) == 0 {
		return map[string]string{}, nil
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	payload := map[string]any{
		"type":   "player",
		"player": playerJS,
		"requests": []map[string]any{
			{
				"type":       challengeType,
				"challenges": challenges,
			},
		},
		"output_preprocessed": false,
	}
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	var script strings.Builder
	script.WriteString(nodeSolverLib)
	script.WriteString("\nObject.assign(globalThis, lib);\n")
	script.WriteString(nodeSolverCore)
	script.WriteString("\nconsole.log(JSON.stringify(jsc(")
	script.Write(payloadJSON)
	script.WriteString(")));\n")

	cmd := exec.CommandContext(ctx, "node", "-")
	cmd.Stdin = strings.NewReader(script.String())
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if message := strings.TrimSpace(stderr.String()); message != "" {
			return nil, fmt.Errorf("%w: %s", err, message)
		}
		return nil, err
	}

	var output struct {
		Type      string `json:"type"`
		Error     string `json:"error"`
		Responses []struct {
			Type  string            `json:"type"`
			Error string            `json:"error"`
			Data  map[string]string `json:"data"`
		} `json:"responses"`
	}
	if err := json.Unmarshal(bytes.TrimSpace(stdout.Bytes()), &output); err != nil {
		return nil, err
	}
	if output.Type == "error" {
		return nil, errors.New(output.Error)
	}
	if len(output.Responses) == 0 {
		return nil, fmt.Errorf("node solver returned no responses")
	}
	response := output.Responses[0]
	if response.Type == "error" {
		return nil, errors.New(response.Error)
	}
	if len(response.Data) == 0 {
		return nil, fmt.Errorf("node solver returned no data")
	}
	return response.Data, nil
}
