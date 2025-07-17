# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is a Go command-line tool that converts JetBrains HTTP Client files (.http) to Postman collections. The tool parses HTTP request files with a specific format and outputs JSON collections that can be imported into Postman.

## Commands

### Build and Run
```bash
go build -o jetbrains-http-to-postman
./jetbrains-http-to-postman input.http output.json
```

### Development
```bash
go run http_to_postman.go input.http output.json
```

### Testing
```bash
go test -v        # Run all tests with verbose output
go test           # Run tests
go test -bench=.  # Run benchmarks
```

Note: One test (`TestOneLineJSON`) is currently failing and needs investigation.

### Code Quality
```bash
go fmt ./...      # Format code
go vet ./...      # Check for common mistakes
```

## Code Architecture

### Core Components

**Main Entry Point**: `http_to_postman.go:60-76`
- CLI argument parsing and error handling
- Calls `convertHTTPToPostman` function

**Parser Core**: `convertHTTPToPostman` function (`http_to_postman.go:78-263`)
- Line-by-line parsing of .http files using `bufio.Scanner`
- State machine approach with flags like `startedJSON`
- Regex-based HTTP method detection
- Sequential processing of headers, body, and URL components

**Data Structures**: `http_to_postman.go:13-58`
- `Collection` - Root Postman collection structure
- `Item` - Individual HTTP request
- `Request` - HTTP request details (method, headers, body, URL)
- `URL` - Parsed URL components (protocol, host, path, query)
- `Header`, `Body`, `QueryParam` - Supporting structures

### Parsing Logic

The parser uses a state machine approach:

1. **HTTP Method Line**: Regex match for `GET|PUT|POST|DELETE|OPTIONS` triggers new request
2. **Header Lines**: Lines with `:` (when not in JSON mode) become headers
3. **JSON Body**: Lines starting with `{` trigger JSON parsing mode
4. **Request Separator**: `###` marks end of request block

### URL Processing

URL parsing splits components:
- Protocol extraction from `protocol://` 
- Host splitting on `.` (e.g., `api.example.com` → `["api", "example", "com"]`)
- Path splitting on `/`
- Query parameter parsing from `?key=value&key2=value2`

### Output Format

Generates Postman Collection v2.1.0 JSON with:
- Timestamped collection names (`jb-export-YYYYMMDDHHMMSS`)
- Individual request items with parsed components
- Proper header and body formatting for JSON requests

## Testing

Comprehensive test suite in `http_to_postman_test.go`:
- Unit tests for various HTTP request formats
- JSON body parsing (single-line and multi-line)
- Query parameter handling
- Multiple request parsing
- Comment filtering
- Error handling for invalid inputs
- Benchmark tests for performance

## Development Notes

- No external dependencies beyond Go standard library
- Single file architecture for simplicity
- Temporary files used extensively in tests
- JSON marshaling/unmarshaling for validation in tests