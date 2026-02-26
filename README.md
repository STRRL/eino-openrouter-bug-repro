# eino + OpenRouter: Tool Message Missing `content` Field

`meguminnnnnnnnn/go-openai` uses `json:"content,omitempty"` on `ChatCompletionMessage.Content`. When a tool returns `""`, the field is omitted, causing API 500.

## Run

```bash
export OPENROUTER_API_KEY=your-key

# reproduce the bug
go run repro.go

# with workaround patch
go run patched.go
```
