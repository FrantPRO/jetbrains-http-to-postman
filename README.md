# jetbrains-http-to-postman
Convert JetBrains HTTP Client files (.http) to Postman collections

## Quick Start

### Run directly:
```bash
go run http_to_postman.go input.http output.json
```

### Build and run:
```bash
go build -o jetbrains-http-to-postman
./jetbrains-http-to-postman input.http output.json
```

## Features

✅ HTTP methods (GET, POST, PUT, DELETE, OPTIONS)
✅ Headers and query parameters
✅ JSON request bodies
✅ Multiple requests per file
✅ Comments support
✅ URL parsing with protocol, host, and path

## Input Format
```http
GET https://api.example.com/users?page=1
Accept: application/json
Authorization: Bearer {{token}}

###

POST https://api.example.com/users
Content-Type: application/json

{
  "name": "John Doe",
  "email": "john@example.com"
}
```
## Output
Generates a Postman collection JSON file that can be imported directly into Postman.

## Development

### Testing
```bash
go test -v        # Run all tests with verbose output
go test           # Run tests
go test -bench=.  # Run benchmarks
```

### Code Quality
```bash
go fmt ./...      # Format code
go vet ./...      # Check for common mistakes
```

## Requirements

Go 1.24.4+

## License
MIT
