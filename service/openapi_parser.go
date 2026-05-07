package service

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
	"gopkg.in/yaml.v3"

	"gopher-mock/model"
)

// ParseOpenAPISpec parses an OpenAPI specification (JSON or YAML) and converts it to mock configurations
func ParseOpenAPISpec(data []byte, isYAML bool) ([]model.MockConfig, error) {
	var loader = openapi3.NewLoader()
	var doc *openapi3.T
	var err error

	if isYAML {
		// Parse YAML
		doc, err = loader.LoadFromData(data)
	} else {
		// Parse JSON
		doc, err = loader.LoadFromData(data)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to parse OpenAPI spec: %w", err)
	}

	// Validate the document
	if err := doc.Validate(loader.Context); err != nil {
		return nil, fmt.Errorf("invalid OpenAPI spec: %w", err)
	}

	var configs []model.MockConfig

	// Iterate through all paths
	for path, pathItem := range doc.Paths.Map() {
		// Convert OpenAPI path parameters to Fiber format (e.g., {id} -> :id)
		fiberPath := convertPathParams(path)

		// Process each HTTP method
		for method, operation := range pathItem.Operations() {
			// Extract all responses and convert to conditional responses
			conditionalResponses, defaultResp := extractAllResponses(operation)

			config := model.MockConfig{
				Name:            generateConfigName(operation, method, path),
				Method:          strings.ToUpper(method),
				Path:            fiberPath,
				RequestHeaders:  extractRequestHeaders(operation),
				RequestBody:     extractRequestBody(operation),
				ResponseHeaders: make(map[string]string),
				ResponseBody:    make(map[string]interface{}),
				StatusCode:      200,
				Timeout:         0,
				Responses:       conditionalResponses,
				DefaultResponse: defaultResp,
			}

			configs = append(configs, config)
		}
	}

	return configs, nil
}

// convertPathParams converts OpenAPI path parameters to Fiber format
// Example: /users/{id} -> /users/:id
func convertPathParams(path string) string {
	result := path
	result = strings.ReplaceAll(result, "{", ":")
	result = strings.ReplaceAll(result, "}", "")
	return result
}

// generateConfigName creates a descriptive and unique name for the mock configuration
func generateConfigName(operation *openapi3.Operation, method, path string) string {
	var baseName string

	if operation.Summary != "" {
		baseName = operation.Summary
	} else if operation.OperationID != "" {
		baseName = operation.OperationID
	} else {
		// Fallback: METHOD /path
		return fmt.Sprintf("%s %s", strings.ToUpper(method), path)
	}

	// Always append method and simplified path to ensure uniqueness
	// Extract last part of path for brevity
	pathParts := strings.Split(strings.Trim(path, "/"), "/")
	lastPart := pathParts[len(pathParts)-1]

	// If last part is a parameter, use the second to last
	if strings.HasPrefix(lastPart, "{") && len(pathParts) > 1 {
		lastPart = pathParts[len(pathParts)-2]
	}

	return fmt.Sprintf("%s [%s %s]", baseName, strings.ToUpper(method), lastPart)
}

// extractRequestHeaders extracts required headers from the operation
func extractRequestHeaders(operation *openapi3.Operation) map[string]string {
	headers := make(map[string]string)

	if operation.Parameters == nil {
		return headers
	}

	for _, paramRef := range operation.Parameters {
		if paramRef.Value == nil {
			continue
		}

		param := paramRef.Value
		if param.In == "header" && param.Required {
			// Use example value if available, otherwise use schema default or empty string
			exampleValue := getExampleValue(param.Schema)
			headers[param.Name] = exampleValue
		}
	}

	return headers
}

// extractRequestBody extracts the request body schema and generates an example
func extractRequestBody(operation *openapi3.Operation) map[string]interface{} {
	if operation.RequestBody == nil || operation.RequestBody.Value == nil {
		return make(map[string]interface{})
	}

	requestBody := operation.RequestBody.Value

	// Try to get JSON content
	content := requestBody.Content.Get("application/json")
	if content == nil {
		return make(map[string]interface{})
	}

	if content.Schema == nil || content.Schema.Value == nil {
		return make(map[string]interface{})
	}

	return generateExampleFromSchema(content.Schema.Value)
}

// extractAllResponses extracts all response codes and converts them to conditional responses
func extractAllResponses(operation *openapi3.Operation) ([]model.ConditionalResponse, *model.Response) {
	if operation.Responses == nil {
		return nil, &model.Response{
			Headers:    make(map[string]string),
			Body:       make(map[string]interface{}),
			StatusCode: 200,
			Timeout:    0,
		}
	}

	var conditionalResponses []model.ConditionalResponse
	var defaultResponse *model.Response

	// Map to track which status codes we've processed
	processedCodes := make(map[int]bool)

	// Process all defined responses
	for statusCodeStr, responseRef := range operation.Responses.Map() {
		if responseRef == nil || responseRef.Value == nil {
			continue
		}

		response := responseRef.Value

		// Convert status code string to int
		var statusCode int
		if statusCodeStr == "default" {
			statusCode = 200
		} else {
			statusCode = statusCodeToInt(statusCodeStr)
		}

		// Skip if already processed
		if processedCodes[statusCode] {
			continue
		}
		processedCodes[statusCode] = true

		// Extract response body
		responseBody := extractResponseBodyFromResponse(response)

		// Create response object
		resp := model.Response{
			Headers:    make(map[string]string),
			Body:       responseBody,
			StatusCode: statusCode,
			Timeout:    0,
		}

		// Determine if this should be the default or a conditional response
		if statusCode >= 200 && statusCode < 300 {
			// Success responses become the default
			if defaultResponse == nil || statusCode == 200 {
				defaultResponse = &resp
			}
		} else {
			// Error responses become conditional responses
			// Create a descriptive name based on status code
			responseName := getResponseNameByStatusCode(statusCode, response.Description)

			// Create a conditional response that triggers on specific status code
			conditionalResp := model.ConditionalResponse{
				Name:         responseName,
				RuleOperator: "AND",
				Rules:        []model.Rule{}, // No rules for now - user can add them manually
				Response:     resp,
			}

			conditionalResponses = append(conditionalResponses, conditionalResp)
		}
	}

	// If no default response was set, create a basic one
	if defaultResponse == nil {
		defaultResponse = &model.Response{
			Headers:    make(map[string]string),
			Body:       make(map[string]interface{}),
			StatusCode: 200,
			Timeout:    0,
		}
	}

	return conditionalResponses, defaultResponse
}

// extractResponseBodyFromResponse extracts body from a response object
func extractResponseBodyFromResponse(response *openapi3.Response) map[string]interface{} {
	if response == nil {
		return make(map[string]interface{})
	}

	// Try to get JSON content
	content := response.Content.Get("application/json")
	if content == nil {
		// No content schema, use description directly
		if response.Description != nil && *response.Description != "" {
			// Return description as the body value
			// For simplicity, just use the description text
			return map[string]interface{}{
				"description": *response.Description,
			}
		}
		return make(map[string]interface{})
	}

	if content.Schema == nil || content.Schema.Value == nil {
		// Has content type but no schema, use description if available
		if response.Description != nil && *response.Description != "" {
			return map[string]interface{}{
				"description": *response.Description,
			}
		}
		return make(map[string]interface{})
	}

	return generateExampleFromSchema(content.Schema.Value)
}

// getResponseNameByStatusCode generates a descriptive name for a response based on status code
func getResponseNameByStatusCode(statusCode int, description *string) string {
	var name string

	switch statusCode {
	case 400:
		name = "Bad Request"
	case 401:
		name = "Unauthorized"
	case 403:
		name = "Forbidden"
	case 404:
		name = "Not Found"
	case 500:
		name = "Internal Server Error"
	default:
		name = fmt.Sprintf("Response %d", statusCode)
	}

	if description != nil && *description != "" {
		name = fmt.Sprintf("%s - %s", name, *description)
	}

	return name
}

// extractResponseBody extracts the response body schema and generates an example
func extractResponseBody(operation *openapi3.Operation) map[string]interface{} {
	if operation.Responses == nil {
		return make(map[string]interface{})
	}

	// Try to get 200 response first, then 201, then any 2xx
	var response *openapi3.Response
	for _, code := range []string{"200", "201", "202", "204"} {
		if resp := operation.Responses.Status(statusCodeToInt(code)); resp != nil {
			response = resp.Value
			break
		}
	}

	if response == nil {
		// Try to get default response
		if operation.Responses.Default() != nil {
			response = operation.Responses.Default().Value
		}
	}

	if response == nil {
		return make(map[string]interface{})
	}

	// Try to get JSON content
	content := response.Content.Get("application/json")
	if content == nil {
		return make(map[string]interface{})
	}

	if content.Schema == nil || content.Schema.Value == nil {
		return make(map[string]interface{})
	}

	return generateExampleFromSchema(content.Schema.Value)
}

// generateExampleFromSchema generates an example object from an OpenAPI schema
func generateExampleFromSchema(schema *openapi3.Schema) map[string]interface{} {
	result := make(map[string]interface{})

	// If there's an example, use it
	if schema.Example != nil {
		if exampleMap, ok := schema.Example.(map[string]interface{}); ok {
			return exampleMap
		}
	}

	// Handle array type at root level
	if len(schema.Type.Slice()) > 0 && schema.Type.Slice()[0] == "array" {
		// For array schemas, return empty array or array with one example item
		if schema.Items != nil && schema.Items.Value != nil {
			itemExample := generateExampleFromSchema(schema.Items.Value)
			// Return as a wrapper to maintain map[string]interface{} return type
			// The caller should handle this appropriately
			result["items"] = []interface{}{itemExample}
			return result
		}
		result["items"] = []interface{}{}
		return result
	}

	// If no properties, return empty object (for responses without body schema)
	if schema.Properties == nil || len(schema.Properties) == 0 {
		return result
	}

	// Generate from properties
	for propName, propSchemaRef := range schema.Properties {
		if propSchemaRef.Value == nil {
			continue
		}

		propSchema := propSchemaRef.Value

		// Check if property has an example
		if propSchema.Example != nil {
			result[propName] = propSchema.Example
			continue
		}

		// Handle empty type slice
		if len(propSchema.Type.Slice()) == 0 {
			result[propName] = ""
			continue
		}

		// Generate based on type
		switch propSchema.Type.Slice()[0] {
		case "string":
			if propSchema.Format == "email" {
				result[propName] = "{{faker.email}}"
			} else if propSchema.Format == "uuid" {
				result[propName] = "{{faker.uuid}}"
			} else if propSchema.Format == "date-time" {
				result[propName] = "{{faker.date}}"
			} else if propSchema.Format == "uri" {
				result[propName] = "{{faker.url}}"
			} else {
				result[propName] = "{{faker.name}}"
			}
		case "integer", "number":
			if propSchema.Example != nil {
				result[propName] = propSchema.Example
			} else if propSchema.Default != nil {
				result[propName] = propSchema.Default
			} else {
				result[propName] = "{{faker.number}}"
			}
		case "boolean":
			if propSchema.Default != nil {
				result[propName] = propSchema.Default
			} else {
				result[propName] = false
			}
		case "array":
			// Handle array items
			if propSchema.Items != nil && propSchema.Items.Value != nil {
				itemExample := generateExampleFromSchema(propSchema.Items.Value)
				result[propName] = []interface{}{itemExample}
			} else {
				result[propName] = []interface{}{}
			}
		case "object":
			if propSchema.Properties != nil {
				result[propName] = generateExampleFromSchema(propSchema)
			} else {
				result[propName] = make(map[string]interface{})
			}
		default:
			result[propName] = ""
		}
	}

	return result
}

// getExampleValue gets an example value from a schema
func getExampleValue(schemaRef *openapi3.SchemaRef) string {
	if schemaRef == nil || schemaRef.Value == nil {
		return ""
	}

	schema := schemaRef.Value

	if schema.Example != nil {
		if str, ok := schema.Example.(string); ok {
			return str
		}
		// Convert to JSON string if not a string
		if bytes, err := json.Marshal(schema.Example); err == nil {
			return string(bytes)
		}
	}

	if schema.Default != nil {
		if str, ok := schema.Default.(string); ok {
			return str
		}
	}

	return ""
}

// statusCodeToInt converts status code string to int
func statusCodeToInt(code string) int {
	switch code {
	case "200":
		return 200
	case "201":
		return 201
	case "202":
		return 202
	case "204":
		return 204
	case "400":
		return 400
	case "401":
		return 401
	case "403":
		return 403
	case "404":
		return 404
	case "500":
		return 500
	default:
		return 200
	}
}

// ParseSwagger2Spec parses a Swagger 2.0 specification
// This is a simplified version that converts Swagger 2.0 to basic mock configs
func ParseSwagger2Spec(data []byte, isYAML bool) ([]model.MockConfig, error) {
	var spec map[string]interface{}
	var err error

	if isYAML {
		err = yaml.Unmarshal(data, &spec)
	} else {
		err = json.Unmarshal(data, &spec)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to parse Swagger spec: %w", err)
	}

	// Check if it's Swagger 2.0
	if swagger, ok := spec["swagger"].(string); !ok || !strings.HasPrefix(swagger, "2.") {
		return nil, fmt.Errorf("not a valid Swagger 2.0 specification")
	}

	var configs []model.MockConfig

	paths, ok := spec["paths"].(map[string]interface{})
	if !ok {
		return configs, nil
	}

	for path, pathItemInterface := range paths {
		pathItem, ok := pathItemInterface.(map[string]interface{})
		if !ok {
			continue
		}

		fiberPath := convertPathParams(path)

		for method, operationInterface := range pathItem {
			// Skip non-method fields
			if method == "parameters" || method == "$ref" {
				continue
			}

			operation, ok := operationInterface.(map[string]interface{})
			if !ok {
				continue
			}

			name := fmt.Sprintf("%s %s", strings.ToUpper(method), path)
			if summary, ok := operation["summary"].(string); ok && summary != "" {
				name = summary
			} else if operationID, ok := operation["operationId"].(string); ok && operationID != "" {
				name = operationID
			}

			config := model.MockConfig{
				Name:            name,
				Method:          strings.ToUpper(method),
				Path:            fiberPath,
				RequestHeaders:  make(map[string]string),
				RequestBody:     make(map[string]interface{}),
				ResponseHeaders: make(map[string]string),
				ResponseBody:    extractSwagger2Response(operation),
				StatusCode:      200,
				Timeout:         0,
				DefaultResponse: &model.Response{
					Headers:    make(map[string]string),
					Body:       extractSwagger2Response(operation),
					StatusCode: 200,
					Timeout:    0,
				},
			}

			configs = append(configs, config)
		}
	}

	return configs, nil
}

// extractSwagger2Response extracts response from Swagger 2.0 operation
func extractSwagger2Response(operation map[string]interface{}) map[string]interface{} {
	responses, ok := operation["responses"].(map[string]interface{})
	if !ok {
		return make(map[string]interface{})
	}

	// Try to get 200 response
	for _, code := range []string{"200", "201", "202"} {
		if response, ok := responses[code].(map[string]interface{}); ok {
			if schema, ok := response["schema"].(map[string]interface{}); ok {
				return generateExampleFromSwagger2Schema(schema)
			}
		}
	}

	return make(map[string]interface{})
}

// generateExampleFromSwagger2Schema generates example from Swagger 2.0 schema
func generateExampleFromSwagger2Schema(schema map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})

	schemaType, _ := schema["type"].(string)

	if schemaType == "object" {
		properties, ok := schema["properties"].(map[string]interface{})
		if !ok {
			return result
		}

		for propName, propInterface := range properties {
			prop, ok := propInterface.(map[string]interface{})
			if !ok {
				continue
			}

			propType, _ := prop["type"].(string)
			switch propType {
			case "string":
				result[propName] = "{{faker.name}}"
			case "integer", "number":
				result[propName] = 0
			case "boolean":
				result[propName] = false
			case "array":
				result[propName] = []interface{}{}
			case "object":
				result[propName] = generateExampleFromSwagger2Schema(prop)
			default:
				result[propName] = ""
			}
		}
	}

	return result
}
