package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

type Collection struct {
	Info  Info   `json:"info"`
	Items []Item `json:"items"`
}

type Info struct {
	Name   string `json:"name"`
	Schema string `json:"schema"`
}

type Item struct {
	Name    string  `json:"name"`
	Request Request `json:"request"`
}

type Request struct {
	Method string   `json:"method"`
	Header []Header `json:"header"`
	Body   Body     `json:"body"`
	URL    URL      `json:"url"`
}

type Header struct {
	Key   string `json:"key"`
	Value string `json:"value"`
	Type  string `json:"type"`
}

type Body struct {
	Mode    string                 `json:"mode,omitempty"`
	Raw     string                 `json:"raw,omitempty"`
	Options map[string]interface{} `json:"options,omitempty"`
}

type URL struct {
	Raw      string       `json:"raw"`
	Protocol string       `json:"protocol,omitempty"`
	Host     []string     `json:"host,omitempty"`
	Path     []string     `json:"path,omitempty"`
	Query    []QueryParam `json:"query,omitempty"`
}

type QueryParam struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type Environment map[string]map[string]string

// detectVariables finds all variables in the format {{variableName}} in the text
func detectVariables(text string) []string {
	varRegex := regexp.MustCompile(`\{\{(\w+)\}\}`)
	matches := varRegex.FindAllStringSubmatch(text, -1)
	var variables []string
	for _, match := range matches {
		if len(match) > 1 {
			variables = append(variables, match[1])
		}
	}
	return variables
}

// loadEnvironment loads the http-client.env.json file from the input file's directory
func loadEnvironment(inputFilePath string) (Environment, error) {
	dir := filepath.Dir(inputFilePath)
	envFile := filepath.Join(dir, "http-client.env.json")

	data, err := os.ReadFile(envFile)
	if err != nil {
		return nil, err
	}

	var env Environment
	err = json.Unmarshal(data, &env)
	if err != nil {
		return nil, fmt.Errorf("failed to parse http-client.env.json: %v", err)
	}

	return env, nil
}

// substituteVariables replaces variables in text with values from the environment
func substituteVariables(text string, env Environment, envName string) string {
	if env == nil || env[envName] == nil {
		return text
	}

	varRegex := regexp.MustCompile(`\{\{(\w+)\}\}`)
	return varRegex.ReplaceAllStringFunc(text, func(match string) string {
		varName := strings.Trim(match, "{}")
		if value, exists := env[envName][varName]; exists {
			return value
		}
		return match // Return original if variable not found
	})
}

func main() {
	if len(os.Args) < 3 {
		fmt.Println("Usage: go run main.go <input.http> <output.json>")
		os.Exit(1)
	}

	inputFile := os.Args[1]
	outputFile := os.Args[2]

	err := convertHTTPToPostman(inputFile, outputFile)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Successfully converted %s to %s\n", inputFile, outputFile)
}

func convertHTTPToPostman(inputFile, outputFile string) error {
	file, err := os.Open(inputFile)
	if err != nil {
		return err
	}
	defer file.Close()

	// First, read the entire file to detect variables
	file.Seek(0, 0) // Reset file pointer
	var fileContent strings.Builder
	fileScanner := bufio.NewScanner(file)
	for fileScanner.Scan() {
		fileContent.WriteString(fileScanner.Text() + "\n")
	}

	// Detect variables in the file content
	allVariables := detectVariables(fileContent.String())

	// Load environment variables
	env, envErr := loadEnvironment(inputFile)

	// Check if variables exist but env file doesn't
	if len(allVariables) > 0 && envErr != nil {
		return fmt.Errorf("variables found in input file (%v) but http-client.env.json is missing or invalid: %v", allVariables, envErr)
	}

	// We'll use "dev" as the default environment name
	envName := "dev"

	// Reset file pointer for actual parsing
	file.Seek(0, 0)

	var items []Item
	var item Item
	var req Request
	var headers []Header
	var body Body
	var url URL
	var query []QueryParam
	var data strings.Builder
	count := 0
	startedJSON := false

	// Initialize first item
	item = Item{Request: Request{}}
	req = Request{
		Header: []Header{},
		Body:   Body{},
		URL:    URL{Query: []QueryParam{}},
	}

	// Regex patterns
	httpMethodRegex := regexp.MustCompile(`^(GET|PUT|POST|DELETE|OPTIONS)\s+.+`)

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		switch {
		case line == "" || strings.HasPrefix(line, "# ") || strings.HasPrefix(line, "//"):
			// Skip comments and empty lines
			continue

		case strings.HasPrefix(line, "###") && !strings.HasPrefix(line, "####"):
			// Save previous request if it exists
			if req.Method != "" && url.Raw != "" {
				req.Header = headers
				req.Body = body
				req.URL = url
				req.URL.Query = query
				item.Request = req
				items = append(items, item)
			}

			// Reset for next request and extract name
			item = Item{Request: Request{}}
			req = Request{
				Header: []Header{},
				Body:   Body{},
				URL:    URL{Query: []QueryParam{}},
			}
			headers = []Header{}
			body = Body{}
			url = URL{}
			query = []QueryParam{}
			data.Reset()
			startedJSON = false

			// Extract request name from ### line
			requestName := strings.TrimSpace(strings.TrimPrefix(line, "###"))
			if requestName == "" {
				count++
				item.Name = fmt.Sprintf("request-%d", count)
			} else {
				item.Name = requestName
			}
			continue

		case httpMethodRegex.MatchString(line):
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				req.Method = parts[0]
				rawURL := parts[1]

				// Substitute variables in URL
				rawURL = substituteVariables(rawURL, env, envName)
				url.Raw = rawURL

				// If no name was set by ###, generate one
				if item.Name == "" {
					count++
					item.Name = fmt.Sprintf("request-%d", count)
				}

				// Parse URL
				if strings.Contains(rawURL, "://") {
					protocolParts := strings.Split(rawURL, "://")
					url.Protocol = protocolParts[0]

					cleanURL := protocolParts[1]
					if strings.Contains(cleanURL, "?") {
						cleanURL = strings.Split(cleanURL, "?")[0]
					}

					if strings.Contains(cleanURL, "/") {
						urlParts := strings.Split(cleanURL, "/")
						url.Host = strings.Split(urlParts[0], ".")
						if len(urlParts) > 1 {
							url.Path = urlParts[1:]
						}
					} else {
						url.Host = strings.Split(cleanURL, ".")
					}
				} else {
					urlParts := strings.Split(rawURL, "/")
					if len(urlParts) > 0 {
						url.Host = strings.Split(urlParts[0], ".")
					}
				}

				// Parse query parameters
				if strings.Contains(rawURL, "?") {
					queryParts := strings.Split(rawURL, "?")
					if len(queryParts) > 1 {
						queryString := queryParts[1]
						params := strings.Split(queryString, "&")
						for _, param := range params {
							if strings.Contains(param, "=") {
								kv := strings.Split(param, "=")
								query = append(query, QueryParam{
									Key:   strings.TrimSpace(kv[0]),
									Value: strings.TrimSpace(kv[1]),
								})
							}
						}
					}
				}
			}

		case strings.HasPrefix(line, "{"):
			// Start of JSON body
			body.Mode = "raw"
			body.Options = map[string]interface{}{
				"raw": map[string]interface{}{
					"language": "json",
				},
			}
			startedJSON = true

			if strings.HasSuffix(line, "}") {
				// Single line JSON
				rawBody := strings.TrimSpace(line)

				// Substitute variables in JSON body
				rawBody = substituteVariables(rawBody, env, envName)
				body.Raw = rawBody
				startedJSON = false
			} else {
				// Multi-line JSON starts
				data.WriteString(line)
				data.WriteString("\n")
			}

		case strings.Contains(line, ":") && !startedJSON:
			// Parse headers
			headerParts := strings.SplitN(line, ":", 2)
			if len(headerParts) == 2 {
				key := strings.TrimSpace(headerParts[0])
				value := strings.TrimSpace(headerParts[1])

				// Substitute variables in header value
				value = substituteVariables(value, env, envName)

				header := Header{
					Key:   key,
					Value: value,
					Type:  "text",
				}
				headers = append(headers, header)
			}

		case strings.Contains(line, ":") && startedJSON:
			// Continue JSON body
			data.WriteString(line + "\n")

		case strings.HasSuffix(line, "}") && startedJSON:
			// End of JSON body
			data.WriteString(line)
			rawBody := strings.TrimSpace(data.String())

			// Substitute variables in JSON body
			rawBody = substituteVariables(rawBody, env, envName)
			body.Raw = rawBody
			startedJSON = false

		case startedJSON:
			// Any other line while parsing JSON
			data.WriteString(line + "\n")

		}
	}

	if err := scanner.Err(); err != nil {
		return err
	}

	// Handle the last request if it doesn't end with ###
	if req.Method != "" && url.Raw != "" {
		req.Header = headers
		req.Body = body
		req.URL = url
		req.URL.Query = query
		item.Request = req
		items = append(items, item)
	}

	// Create collection
	today := time.Now().Format("20060102150405")
	collection := Collection{
		Info: Info{
			Name:   fmt.Sprintf("jb-export-%s", today),
			Schema: "https://schema.getpostman.com/json/collection/v2.1.0/collection.json",
		},
		Items: items,
	}

	// Write output file
	output, err := json.MarshalIndent(collection, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(outputFile, output, 0644)
}
