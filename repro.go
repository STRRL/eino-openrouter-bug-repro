//go:build ignore

package main

import (
	"context"
	"fmt"
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

	chatModel, _ := openrouter.NewChatModel(ctx, &openrouter.Config{
		APIKey: apiKey,
		Model:  "anthropic/claude-sonnet-4.6",
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
