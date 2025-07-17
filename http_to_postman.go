package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
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
	Method string            `json:"method"`
	Header []Header          `json:"header"`
	Body   Body              `json:"body"`
	URL    URL               `json:"url"`
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
		case strings.HasPrefix(line, "# "):
			// Skip comments
			continue

		case httpMethodRegex.MatchString(line):
			count++
			item.Name = fmt.Sprintf("request-%d", count)
			
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				req.Method = parts[0]
				rawURL := parts[1]
				url.Raw = rawURL

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

		case strings.Contains(line, ":") && !startedJSON:
			// Parse headers
			headerParts := strings.SplitN(line, ":", 2)
			if len(headerParts) == 2 {
				header := Header{
					Key:   strings.TrimSpace(headerParts[0]),
					Value: strings.TrimSpace(headerParts[1]),
					Type:  "text",
				}
				headers = append(headers, header)
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
			data.WriteString(line + "\n")
			
			if strings.HasSuffix(line, "}") {
				body.Raw = strings.TrimSpace(data.String())
				startedJSON = false
			}

		case strings.Contains(line, ":") && startedJSON:
			// Continue JSON body
			data.WriteString(line + "\n")

		case strings.HasSuffix(line, "}") && startedJSON:
			// End of JSON body
			data.WriteString(line)
			body.Raw = strings.TrimSpace(data.String())
			startedJSON = false

		case strings.HasPrefix(line, "###"):
			// End of request - save current item and reset
			req.Header = headers
			req.Body = body
			req.URL = url
			req.URL.Query = query
			item.Request = req
			items = append(items, item)

			// Reset for next request
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
		}
	}

	if err := scanner.Err(); err != nil {
		return err
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
