**This was built using AI**

# Mock MCP Server

A streamable HTTP-based Model Context Protocol (MCP) server written in Go that mocks tool calls for testing and development purposes.

## Features

- **HTTP/HTTPS Support**: Standard HTTP POST requests for MCP protocol
- **Streaming Support**: Server-Sent Events (SSE) for streaming responses
- **WebSocket Support**: Full WebSocket support for bidirectional communication
- **Mock Tools**: Pre-configured mock tools for testing
- **Dynamic Tool Management**: Add/remove tools at runtime by editing a YAML file
- **Hot Reload**: Automatically reloads tools when the YAML configuration file changes
- **Extensible**: Easy to add custom mock tools via YAML configuration

## Installation

```bash
go mod download
```

## Running the Server

```bash
go run ./cmd/mock-mcp/main.go
```

Or build and run:

```bash
go build -o bin/mock-mcp-server ./cmd/mock-mcp
./bin/mock-mcp-server
```

The server will start on port 8080 by default.

### Configuration File

The server uses a YAML configuration file (`config/tools.yaml` by default) to manage tools. You can specify a custom path using the `TOOLS_CONFIG` environment variable:

```bash
TOOLS_CONFIG=/path/to/custom-tools.yaml go run ./cmd/mock-mcp/main.go
```

The server automatically watches the configuration file and reloads tools when changes are detected. No restart required!

## Docker

### Building the Docker Image

```bash
docker build -t mock-mcp-server:latest .
```

### Running with Docker

#### Using Docker Run

```bash
docker run -d \
  --name mock-mcp-server \
  -p 8080:8080 \
  -v $(pwd)/config:/app/config:ro \
  -v $(pwd)/testcases:/app/testcases:ro \
  -e TOOLS_CONFIG=/app/config/tools.yaml \
  mock-mcp-server:latest
```

#### Using Docker Compose (Recommended)

```bash
docker-compose up -d
```

This will:
- Build the image if needed
- Start the container with proper volume mounts
- Mount `./config` to `/app/config` (read-only)
- Mount `./testcases` to `/app/testcases` (read-only)
- Expose port 8080

To stop:

```bash
docker-compose down
```

### Volume Mounts

The Docker setup mounts two volumes:

1. **Config Volume**: `./config` → `/app/config`
   - Contains `tools.yaml` with tool definitions
   - Mounted as read-only for safety
   - Changes to `tools.yaml` are automatically detected and reloaded

2. **Testcases Volume**: `./testcases` → `/app/testcases`
   - Contains all test case YAML files
   - Mounted as read-only
   - Test cases are loaded from this directory

### Custom Volume Paths

You can customize the volume paths in `docker-compose.yml`:

```yaml
volumes:
  - /path/to/your/config:/app/config:ro
  - /path/to/your/testcases:/app/testcases:ro
```

### Environment Variables

- `TOOLS_CONFIG`: Path to the tools configuration file (default: `/app/config/tools.yaml`)

### Health Check

The container includes a health check that monitors the `/health` endpoint. You can check the container status:

```bash
docker ps
```

## Project Structure

```
mock-mcp/
├── cmd/
│   └── mock-mcp/          # Application entry point
│       └── main.go
├── internal/
│   └── mcp/               # Internal MCP server package
│       ├── types.go        # Type definitions
│       ├── server.go       # HTTP server and MCP protocol handlers
│       ├── tools.go        # Tool management and YAML loading
│       └── testcases.go    # Test case loading and matching
├── config/
│   └── tools.yaml         # Tool definitions
├── testcases/             # Test case YAML files
│   ├── mock_echo-test-case-1.yaml
│   ├── mock_calculator-test-case-1.yaml
│   └── ...
├── scripts/               # Utility scripts
│   └── example.sh         # Example test script
├── bin/                   # Build output (gitignored)
├── Dockerfile             # Docker build configuration
├── docker-compose.yml     # Docker Compose configuration
├── Makefile               # Build automation
├── go.mod
├── go.sum
└── README.md
```

## Endpoints

- `POST /mcp` - Standard MCP protocol endpoint
- `GET /mcp?stream=true` - Streaming MCP endpoint (Server-Sent Events)
- `WS /mcp` - WebSocket MCP endpoint
- `GET /health` - Health check endpoint

## Usage Examples

### Initialize the MCP Connection

```bash
curl -X POST http://localhost:8080/mcp \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "id": 1,
    "method": "initialize",
    "params": {
      "protocolVersion": "2024-11-05",
      "capabilities": {},
      "clientInfo": {
        "name": "test-client",
        "version": "1.0.0"
      }
    }
  }'
```

### List Available Tools

```bash
curl -X POST http://localhost:8080/mcp \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "id": 2,
    "method": "tools/list"
  }'
```

### Call a Mock Tool

```bash
curl -X POST http://localhost:8080/mcp \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "id": 3,
    "method": "tools/call",
    "params": {
      "name": "mock_echo",
      "arguments": {
        "message": "Hello, MCP!"
      }
    }
  }'
```

### Streaming Tool Call

```bash
curl -X POST http://localhost:8080/mcp?stream=true \
  -H "Content-Type: application/json" \
  -H "Accept: text/event-stream" \
  -d '{
    "jsonrpc": "2.0",
    "id": 4,
    "method": "tools/call",
    "params": {
      "name": "mock_calculator",
      "arguments": {
        "operation": "add",
        "a": 10,
        "b": 5
      }
    }
  }'
```

## Available Mock Tools

### mock_echo
Echoes back the input message.

**Parameters:**
- `message` (string, required): The message to echo

**Example:**
```json
{
  "name": "mock_echo",
  "arguments": {
    "message": "Hello, World!"
  }
}
```

### mock_calculator
Performs basic arithmetic operations.

**Parameters:**
- `operation` (string, required): The operation to perform (add, subtract, multiply, divide)
- `a` (number, required): First number
- `b` (number, required): Second number

**Example:**
```json
{
  "name": "mock_calculator",
  "arguments": {
    "operation": "multiply",
    "a": 6,
    "b": 7
  }
}
```

### mock_delay
Simulates a delayed operation.

**Parameters:**
- `seconds` (number, required): Number of seconds to delay

**Example:**
```json
{
  "name": "mock_delay",
  "arguments": {
    "seconds": 2
  }
}
```

### mock_greeter
Greets a person by name in multiple languages.

**Parameters:**
- `name` (string, required): The name of the person to greet
- `language` (string, optional): Language for the greeting (en, es, fr, de). Defaults to "en"

**Example:**
```json
{
  "name": "mock_greeter",
  "arguments": {
    "name": "Alice",
    "language": "es"
  }
}
```

## YAML Configuration

Tools can be dynamically added, removed, or modified by editing the `tools.yaml` file. The server automatically watches for changes and reloads tools without requiring a restart.

### Configuration File Format

```yaml
tools:
  - name: my_tool
    description: "Description of my tool"
    defaultTestCase: 1  # Optional: Use test-case-1.yaml as default if no match found (0 = no default, omit to disable)
    inputSchema:
      type: object
      properties:
        param1:
          type: string
          description: "Parameter description"
      required:
        - param1
```

### Default Test Case Configuration

You can configure which test case to use as a fallback when no matching test case is found by setting `defaultTestCase` in the tool definition:

- `defaultTestCase: 0` or omitted - No default, returns an error if no match is found
- `defaultTestCase: 1` - Use `tool-name-test-case-1.yaml` as default
- `defaultTestCase: 2` - Use `tool-name-test-case-2.yaml` as default
- etc.

**Example:**
```yaml
tools:
  - name: mock_calculator
    description: "Performs basic arithmetic operations"
    defaultTestCase: 1  # Use test-case-1.yaml as default for unmatched inputs
    inputSchema:
      # ... schema definition
```

This is useful when you want to provide a generic response for inputs that don't match any specific test case.

### Example Configuration

See `config/tools.yaml` for a complete example with all available tools.

### Adding a New Tool

1. Edit `tools.yaml` and add your tool definition:
```yaml
tools:
  - name: my_new_tool
    description: "My new tool"
    inputSchema:
      type: object
      properties:
        input:
          type: string
          description: "Input parameter"
      required:
        - input
```

2. Save the file - the server will automatically reload the tools.

3. Add the execution logic in `executeMockTool()` function in `main.go`:
```go
case "my_new_tool":
    input, _ := args["input"].(string)
    return ToolResult{
        Content: []ContentBlock{
            {
                Type: "text",
                Text: fmt.Sprintf("Processed: %s", input),
            },
        },
    }
```

4. Rebuild the server (only needed for execution logic changes):
```bash
go build -o mock-mcp-server main.go
```

### Removing a Tool

Simply remove the tool entry from `tools.yaml` and save. The tool will be automatically removed from the server.

## Test Cases

The server uses YAML test case files to provide pre-canned responses for tool calls. Instead of executing code, tools return responses from matching test case files.

### Test Case File Format

Test case files are named `<TOOL_NAME>-test-case-X.yaml` where `X` is a number (1, 2, 3, etc.). For example:
- `mock_echo-test-case-1.yaml`
- `mock_calculator-test-case-2.yaml`
- `mock_greeter-test-case-3.yaml`

Each test case file contains:

```yaml
input:
  param1: "value1"
  param2: 42

response:
  content:
    - type: text
      text: "Response text here"
  isError: false
```

### How Test Cases Are Matched

1. The server searches for test case files in the `testcases/` directory (relative to the config file location)
2. It tries test cases in order (1, 2, 3, ...) up to 100
3. For each test case, it compares the `input` section with the actual tool call arguments
4. The first matching test case is used
5. If no match is found, it falls back to `test-case-1.yaml` as a default (if it exists)
6. If no test cases exist, an error is returned

### Example Test Case

**File: `mock_echo-test-case-1.yaml`**
```yaml
input:
  message: "Hello, World!"

response:
  content:
    - type: text
      text: "Echo: Hello, World!"
  isError: false
```

When a tool is called with `{"message": "Hello, World!"}`, this test case will match and return the pre-canned response.

### Creating Test Cases

1. Create a YAML file named `<tool-name>-test-case-X.yaml` in the `testcases/` directory
2. Define the `input` section with the expected arguments
3. Define the `response` section with the desired output
4. Save the file - no restart needed!

### Multiple Test Cases

You can create multiple test cases for the same tool to handle different input scenarios:

- `mock_calculator-test-case-1.yaml` - for addition
- `mock_calculator-test-case-2.yaml` - for multiplication
- `mock_calculator-test-case-3.yaml` - for division by zero error

The server will automatically match the appropriate test case based on the input arguments.

## Protocol

This server implements the Model Context Protocol (MCP) specification. All requests and responses follow the JSON-RPC 2.0 format.

### Request Format

```json
{
  "jsonrpc": "2.0",
  "id": <request_id>,
  "method": "<method_name>",
  "params": { ... }
}
```

### Response Format

```json
{
  "jsonrpc": "2.0",
  "id": <request_id>,
  "result": { ... }
}
```

Or in case of error:

```json
{
  "jsonrpc": "2.0",
  "id": <request_id>,
  "error": {
    "code": <error_code>,
    "message": "<error_message>"
  }
}
```

## Development

### Adding Custom Mock Tools

**Recommended Approach (YAML Configuration):**

1. Add the tool definition to `tools.yaml` (see [YAML Configuration](#yaml-configuration) section above)
2. Create test case YAML files (see [Test Cases](#test-cases) section above)
3. No code changes needed! The server reads responses from test case files automatically

**Note:** The server no longer executes code for tools. All tool responses come from YAML test case files. This makes it easy to:
- Test different scenarios without code changes
- Version control test cases
- Share test cases across environments
- Modify responses without rebuilding

### Workflow Summary

1. **Define tools** in `tools.yaml` - specifies tool names, descriptions, and input schemas
2. **Create test cases** as `<tool-name>-test-case-X.yaml` files - defines input/output pairs
3. **Call tools** via the MCP API - server automatically matches inputs to test cases and returns pre-canned responses

No code changes or rebuilds needed when adding new tools or test cases!

## License

MIT

