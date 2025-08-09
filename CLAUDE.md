# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build and Development Commands

### Building
```bash
# Build the Docker image
docker network create aquarium
docker build -t aquarium .

# Build the Go binary
go build
```

### Running
```bash
# Clean logs before running (recommended)
./cleanup.sh

# Run with OpenAI (requires OPENAI_API_KEY environment variable)
OPENAI_API_KEY=$OPENAI_API_KEY ./aquarium --goal "Your goal is to run a Minecraft server."

# Run with local model (requires llama-cpp-python server on localhost:8000)
./aquarium --goal "Your goal is to run a Minecraft server." --url "http://localhost:8000" --context-mode full

# Run in screen session to avoid TTY issues (recommended)
screen -S aquarium bash -c 'OPENAI_API_KEY=$OPENAI_API_KEY ./aquarium --goal "Your goal here"'
```

### Key Command Line Options
- `--goal`: The objective to give the AI (default: "Your goal is to run a Minecraft server.")
- `--context-mode`: "partial" (last 10 lines) or "full" (entire output) - affects cost vs accuracy
- `--limit`: Maximum number of commands the AI should run (default: 30)
- `--debug`: Enable logging of AI prompts to debug.log
- `--preserve-container`: Keep Docker container after program completes
- `--model`: OpenAI model to use (default: "gpt-4o-mini")
- `--url`: URL to locally hosted LLM endpoint

## Architecture Overview

### Core Components
- **main.go**: TUI application using Charmbracelet's Bubble Tea, displays split-pane interface with logs (left) and terminal output (right)
- **actor/actor.go**: The orchestration engine that manages the Docker container lifecycle, executes AI-generated commands, and handles process tracking
- **ai/openai.go**: LLM integration layer supporting both OpenAI API and local models via llama-cpp-python
- **logger/logger.go**: Multi-channel logging system that writes to files and TUI simultaneously

### Data Flow
1. **Actor Loop**: Creates Ubuntu Docker container → Establishes terminal connection → Begins iteration cycle
2. **AI Interaction**: Sends goal + command history to LLM → Receives Linux command → Sanitizes command
3. **Execution**: Executes command in container → Tracks process completion via PID → Captures output
4. **Output Processing**: Sanitizes terminal output → Handles long outputs via chunking/summarization → Feeds back to AI

### Key Design Patterns
- **Process Tracking**: Uses PID files (`/tmp/last.pid`) and `/proc` filesystem to determine when commands complete
- **Command Sanitization**: Automatically adds `-y` flags to apt commands, quiets verbose output (wget -nv, removes tar -v)
- **Output Chunking**: Long command outputs are recursively split and summarized to fit LLM context windows
- **Dual Logging**: Separate channels for general logs (`aquarium.log`) and terminal output (`terminal.log`)

### Container Configuration
- **Base Image**: Ubuntu 22.04 with sudo, psmisc, colorized-logs
- **User Setup**: Non-root `ubuntu` user with passwordless sudo
- **Terminal Handling**: Uses `script(1)` to capture all terminal activity to `/tmp/out`, processed by `ansi2txt`
- **Network**: Runs on custom `aquarium` Docker network

### LLM Integration Details
- **OpenAI Mode**: Uses chat completions API with temperature=0.0, 200 max tokens
- **Local Mode**: Communicates with llama-cpp-python server using completions endpoint
- **Context Management**: Maintains conversation history as CommandPair structs (command + outcome)
- **Error Handling**: Implements recursive output chunking when context limits are exceeded

## Important Implementation Notes

### Security Considerations
- Container runs with `apparmor:unconfined` for compatibility
- Commands are executed in isolated Docker environment
- No direct filesystem access outside container

### Terminal Output Processing
- Replaces `\r` with `\n` for progress bar compatibility
- Deduplicates consecutive identical lines
- Strips whitespace-only lines
- Cleans up process tracking command artifacts in display

### Error Recovery
- AI API errors trigger graceful shutdown
- Container communication failures are logged and cause actor termination
- Output too large for single API call triggers recursive chunking with exponential backoff

## Development Tips

### Code Formatting
```bash
# Format all Go code in the repository
gofmt -w .
```

### Log Management
```bash
# Clean up log files before running (recommended)
./cleanup.sh
```

### Screen Session Management
```bash
# List active screen sessions
screen -list

# Attach to a running session
screen -r session_name

# Detach from current session
Ctrl+A, then D

# Kill a session
screen -S session_name -X quit

# IMPORTANT: Always clean up screen sessions when done!
```

### Troubleshooting
- **TTY Issues**: Use screen sessions when running from non-interactive environments
- **"exec: cd: not found"**: Fixed in current version - shell builtins now handled correctly
- **Markdown formatting**: AI responses are automatically cleaned of ```bash code blocks
- **Long outputs**: Use `--context-mode full` for better AI understanding at higher cost

## Log Files
- `aquarium.log`: General application logs and actor state
- `terminal.log`: Current terminal state as seen by the AI
- `debug.log`: API requests/responses (only when `--debug` flag used)
- `cleanup.sh`: Script to safely remove all three log files