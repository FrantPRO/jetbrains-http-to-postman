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
	Info     Info       `json:"info"`
	Items    []Item     `json:"item"`
	Variable []Variable `json:"variable"`
}

type Info struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Schema      string `json:"schema"`
}

type Item struct {
	Name        string  `json:"name"`
	Description string  `json:"description,omitempty"`
	Item        []Item  `json:"item,omitempty"`
	Request     Request `json:"request,omitempty"`
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
	Variable []Variable   `json:"variable,omitempty"`
}

type QueryParam struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type Variable struct {
	Key   string `json:"key"`
	Value string `json:"value"`
	Type  string `json:"type,omitempty"`
}

type Group struct {
	Name  string
	Items []Item
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

// resetRequest resets all request-related variables
func resetRequest(item *Item, req *Request, headers *[]Header, body *Body, url *URL, query *[]QueryParam, data *strings.Builder, startedJSON *bool, currentRequestName *string, requestVars *map[string]string, inRequestScript *bool) {
	*item = Item{}
	*req = Request{
		Header: []Header{},
		Body:   Body{},
		URL:    URL{Query: []QueryParam{}},
	}
	*headers = []Header{}
	*body = Body{}
	*url = URL{Variable: []Variable{}}
	*query = []QueryParam{}
	data.Reset()
	*startedJSON = false
	*currentRequestName = ""
	*requestVars = make(map[string]string)
	*inRequestScript = false
}

// parseURL parses a URL and sets the appropriate fields for Postman format
func parseURL(rawURL string, url *URL, localVars map[string]string, requestVars map[string]string) {
	var pathVariables []Variable
	varRegex := regexp.MustCompile(`\{\{(\w+)\}\}`)

	// Handle variables in URL by keeping them as-is
	if strings.Contains(rawURL, "{{baseUrl}}") || strings.Contains(rawURL, "{{baseURL}}") {
		// For URLs with baseUrl variable, set host to the variable
		url.Host = []string{"{{baseUrl}}"}
		// Parse the path part - split by / and remove query params
		pathPart := rawURL
		if strings.Contains(pathPart, "?") {
			pathPart = strings.Split(pathPart, "?")[0]
		}

		if strings.Contains(pathPart, "/") {
			parts := strings.Split(pathPart, "/")
			if len(parts) > 1 {
				var pathParts []string
				for i, part := range parts {
					if i == 0 {
						continue // Skip the baseUrl part
					}
					if part != "" {
						// Check if this path segment contains variables
						if varRegex.MatchString(part) {
							convertedPart := part
							matches := varRegex.FindAllStringSubmatch(part, -1)
							for _, match := range matches {
								if len(match) > 1 {
									varName := match[1]
									// Convert {{variable}} to :variable for path
									convertedPart = strings.ReplaceAll(convertedPart, match[0], ":"+varName)

									// Determine variable value from different scopes
									varValue := ""
									if requestVars != nil {
										if val, exists := requestVars[varName]; exists {
											varValue = val
										}
									}
									if varValue == "" {
										if val, exists := localVars[varName]; exists {
											varValue = val
										}
									}

									// Add to path variables
									pathVariables = append(pathVariables, Variable{
										Key:   varName,
										Value: varValue,
										Type:  "string",
									})
								}
							}
							pathParts = append(pathParts, convertedPart)
						} else {
							pathParts = append(pathParts, part)
						}
					}
				}
				url.Path = pathParts
			}
		}

		if len(pathVariables) > 0 {
			url.Variable = pathVariables
		}
		return
	}

	// Standard URL parsing
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

	// Storage for local variables (@var_name = value)
	localVariables := make(map[string]string)

	// Reset file pointer for actual parsing
	file.Seek(0, 0)

	var groups []Group
	var currentGroup *Group
	var allItems []Item
	var hasGroupSeparators bool
	var item Item
	var req Request
	var headers []Header
	var body Body
	var url URL
	var query []QueryParam
	var data strings.Builder
	count := 0
	startedJSON := false
	var currentRequestName string
	var currentRequestDescription string
	var requestVariables map[string]string // Request-level variables for current request
	var inRequestScript bool               // Flag to track if we're inside a request script block

	// Initialize first item
	item = Item{}
	req = Request{
		Header: []Header{},
		Body:   Body{},
		URL:    URL{Query: []QueryParam{}, Variable: []Variable{}},
	}
	requestVariables = make(map[string]string)

	// Regex patterns
	httpMethodRegex := regexp.MustCompile(`^(GET|PUT|POST|DELETE|OPTIONS)\s+.+`)
	groupRegex := regexp.MustCompile(`^#\s*@group_name\s+(.+)$`)
	nameRegex := regexp.MustCompile(`^#\s*@name\s+(\w+)$`)
	descriptionRegex := regexp.MustCompile(`^//\s*(.+)$`)
	requestSeparatorRegex := regexp.MustCompile(`^###.*$`)
	localVariableRegex := regexp.MustCompile(`^@(\w+)\s*=\s*(.+)$`)
	requestVariableRegex := regexp.MustCompile(`request\.variables\.set\("([^"]+)",\s*"([^"]+)"\)`)

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		switch {
		case line == "":
			// Skip empty lines
			continue

		case groupRegex.MatchString(line):
			// Group definition: # @group_name PRODUCTS
			matches := groupRegex.FindStringSubmatch(line)
			if len(matches) > 1 {
				groupName := strings.TrimSpace(matches[1])

				// Add current group to groups if it has items
				if currentGroup != nil && len(currentGroup.Items) > 0 {
					groups = append(groups, *currentGroup)
				}

				currentGroup = &Group{Name: groupName, Items: []Item{}}
				hasGroupSeparators = true
			}
			continue

		case nameRegex.MatchString(line):
			// Extract @name comment
			matches := nameRegex.FindStringSubmatch(line)
			if len(matches) > 1 {
				currentRequestName = matches[1]
			}
			continue

		case descriptionRegex.MatchString(line):
			// Extract description from // comment
			matches := descriptionRegex.FindStringSubmatch(line)
			if len(matches) > 1 {
				currentRequestDescription = matches[1]
			}
			continue

		case localVariableRegex.MatchString(line):
			// Parse local variable: @var_name = value
			matches := localVariableRegex.FindStringSubmatch(line)
			if len(matches) > 2 {
				varName := matches[1]
				varValue := strings.TrimSpace(matches[2])
				localVariables[varName] = varValue
			}
			continue

		case strings.HasPrefix(line, "<") && strings.Contains(line, "{%"):
			// Start of request script block (single line or multi-line)
			if strings.Contains(line, "%}") {
				// Single line script block: < {% ... %}
				if requestVariableRegex.MatchString(line) {
					matches := requestVariableRegex.FindStringSubmatch(line)
					if len(matches) > 2 {
						varName := matches[1]
						varValue := matches[2]
						requestVariables[varName] = varValue
					}
				}
			} else {
				// Multi-line script block starts
				inRequestScript = true
				if requestVariableRegex.MatchString(line) {
					matches := requestVariableRegex.FindStringSubmatch(line)
					if len(matches) > 2 {
						varName := matches[1]
						varValue := matches[2]
						requestVariables[varName] = varValue
					}
				}
			}
			continue

		case strings.Contains(line, "%}") && inRequestScript:
			// End of multi-line request script block
			inRequestScript = false
			if requestVariableRegex.MatchString(line) {
				matches := requestVariableRegex.FindStringSubmatch(line)
				if len(matches) > 2 {
					varName := matches[1]
					varValue := matches[2]
					requestVariables[varName] = varValue
				}
			}
			continue

		case inRequestScript:
			// Inside multi-line request script block - check for variable assignments
			if requestVariableRegex.MatchString(line) {
				matches := requestVariableRegex.FindStringSubmatch(line)
				if len(matches) > 2 {
					varName := matches[1]
					varValue := matches[2]
					requestVariables[varName] = varValue
				}
			}
			continue

		case requestSeparatorRegex.MatchString(line):
			// End of request: ### - save current request
			if req.Method != "" && url.Raw != "" {
				req.Header = headers
				req.Body = body
				req.URL = url
				req.URL.Query = query
				item.Request = req

				// Set name and description
				if currentRequestName != "" {
					item.Name = currentRequestName
				} else {
					count++
					item.Name = fmt.Sprintf("request-%d", count)
				}

				if currentRequestDescription != "" {
					item.Description = currentRequestDescription
				}

				// Add to group or all items
				if hasGroupSeparators && currentGroup != nil {
					currentGroup.Items = append(currentGroup.Items, item)
				} else {
					allItems = append(allItems, item)
				}
			}

			// Reset for next request
			resetRequest(&item, &req, &headers, &body, &url, &query, &data, &startedJSON, &currentRequestName, &requestVariables, &inRequestScript)
			currentRequestDescription = ""
			continue

		case strings.HasPrefix(line, "#"):
			// Skip other comments
			continue

		case httpMethodRegex.MatchString(line):
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				req.Method = parts[0]
				rawURL := parts[1]

				// Parse URL and convert to Postman format
				url.Raw = rawURL
				parseURL(rawURL, &url, localVariables, requestVariables)

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
				body.Raw = strings.TrimSpace(line)
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
			body.Raw = strings.TrimSpace(data.String())
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

		// Set name and description
		if currentRequestName != "" {
			item.Name = currentRequestName
		} else {
			count++
			item.Name = fmt.Sprintf("request-%d", count)
		}

		if currentRequestDescription != "" {
			item.Description = currentRequestDescription
		}

		if hasGroupSeparators && currentGroup != nil {
			currentGroup.Items = append(currentGroup.Items, item)
		} else {
			allItems = append(allItems, item)
		}
	}

	// Add the current group to groups if it has items
	if currentGroup != nil && len(currentGroup.Items) > 0 {
		groups = append(groups, *currentGroup)
	}

	// Create collection variables from detected variables
	var collectionVariables []Variable
	uniqueVars := make(map[string]bool)

	// Add detected variables from file content
	for _, varName := range allVariables {
		if !uniqueVars[varName] {
			uniqueVars[varName] = true
			value := ""

			// Check local variables first
			if localValue, exists := localVariables[varName]; exists {
				value = localValue
			} else if env != nil && env[envName] != nil {
				// Then check environment variables
				if val, exists := env[envName][varName]; exists {
					value = val
				}
			}

			collectionVariables = append(collectionVariables, Variable{
				Key:   varName,
				Value: value,
				Type:  "string",
			})
		}
	}

	// Add any local variables that weren't detected in the content
	for varName, varValue := range localVariables {
		if !uniqueVars[varName] {
			collectionVariables = append(collectionVariables, Variable{
				Key:   varName,
				Value: varValue,
				Type:  "string",
			})
		}
	}

	// Note: Request-level variables are handled per-request and not added to global collection variables
	// They are applied during URL and content processing for each specific request

	// Convert groups to Postman folder structure
	var items []Item

	// If we have groups, create folder structure
	if hasGroupSeparators && len(groups) > 0 {
		for _, group := range groups {
			if len(group.Items) > 0 {
				// Filter out any empty request items
				var validItems []Item
				for _, item := range group.Items {
					if item.Request.Method != "" {
						validItems = append(validItems, item)
					}
				}

				if len(validItems) > 0 {
					groupItem := Item{
						Name: group.Name,
						Item: validItems,
						// Don't include Request for group items
					}
					items = append(items, groupItem)
				}
			}
		}
	} else {
		// No groups, add items directly
		for _, item := range allItems {
			if item.Request.Method != "" {
				items = append(items, item)
			}
		}
	}

	// Create collection
	today := time.Now().Format("20060102150405")
	collection := Collection{
		Info: Info{
			Name:   fmt.Sprintf("jb-export-%s", today),
			Schema: "https://schema.getpostman.com/json/collection/v2.1.0/collection.json",
		},
		Items:    items,
		Variable: collectionVariables,
	}

	// Write output file
	output, err := json.MarshalIndent(collection, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(outputFile, output, 0644)
}
