.PHONY: build clean agent mcp

build: agent mcp

agent:
	go build -ldflags="-s -w" -o build/lokifix-agent.exe ./cmd/lokifix-agent/

mcp:
	go build -ldflags="-s -w" -o build/lokifix-mcp.exe ./cmd/lokifix-mcp/

clean:
	rm -rf build/

# Cross-compile for Windows ARM64
agent-arm64:
	GOOS=windows GOARCH=arm64 go build -ldflags="-s -w" -o build/lokifix-agent-arm64.exe ./cmd/lokifix-agent/
