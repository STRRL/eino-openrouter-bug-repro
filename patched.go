//go:build ignore

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/adk/filesystem"
	fsmw "github.com/cloudwego/eino/adk/middlewares/filesystem"
	"github.com/cloudwego/eino-ext/components/model/openrouter"
)

func main() {
	ctx := context.Background()
	apiKey := os.Getenv("OPENROUTER_API_KEY")
	if apiKey == "" {
		fmt.Fprintln(os.Stderr, "OPENROUTER_API_KEY is required")
		os.Exit(1)
	}

	backend := filesystem.NewInMemoryBackend()
	_ = backend.Write(ctx, &filesystem.WriteRequest{
		FilePath: "/app.log",
		Content:  "INFO server started\nINFO request handled\nINFO server stopped\n",
	})

	// only difference: inject fixupTransport to patch missing "content" on tool messages
	chatModel, _ := openrouter.NewChatModel(ctx, &openrouter.Config{
		APIKey: apiKey,
		Model:  "anthropic/claude-sonnet-4.6",
		HTTPClient: &http.Client{
			Transport: &fixupTransport{base: http.DefaultTransport},
		},
	})

	middleware, _ := fsmw.NewMiddleware(ctx, &fsmw.Config{Backend: backend})

	agent, _ := adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
		Name:          "repro-agent",
		Description:   "Bug reproduction agent",
		Instruction:   "You have filesystem tools. Read /app.log, then grep for ERROR in it.",
		Model:         chatModel,
		Middlewares:    []adk.AgentMiddleware{middleware},
		MaxIterations: 5,
	})

	runner := adk.NewRunner(ctx, adk.RunnerConfig{Agent: agent})
	iter := runner.Query(ctx, "Read /app.log and grep for ERROR patterns. Report findings.")

	for {
		event, ok := iter.Next()
		if !ok {
			break
		}
		if event.Err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: %v\n", event.Err)
			break
		}
		msg, _, _ := adk.GetMessage(event)
		if msg != nil && msg.Content != "" {
			fmt.Print(msg.Content)
		}
	}
	fmt.Println()
}

// fixupTransport patches tool messages that are missing the "content" field.
// go-openai uses `json:"content,omitempty"` which omits empty string content,
// but the API requires it on tool messages.
type fixupTransport struct{ base http.RoundTripper }

func (t *fixupTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.Body == nil || req.Method != http.MethodPost {
		return t.base.RoundTrip(req)
	}

	body, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, err
	}
	req.Body.Close()

	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err == nil {
		if msgs, ok := payload["messages"].([]any); ok {
			for _, m := range msgs {
				msg, ok := m.(map[string]any)
				if !ok {
					continue
				}
				if msg["role"] == "tool" {
					if _, hasContent := msg["content"]; !hasContent {
						msg["content"] = ""
					}
				}
			}
			body, _ = json.Marshal(payload)
		}
	}

	req.Body = io.NopCloser(bytes.NewReader(body))
	req.ContentLength = int64(len(body))
	return t.base.RoundTrip(req)
}
