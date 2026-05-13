package handler

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog"

	"gopher-mock/config"
	"gopher-mock/model"
	"gopher-mock/service"
	"gopher-mock/templates"
)

var logIDCounter uint64

func nextLogID() string {
	n := atomic.AddUint64(&logIDCounter, 1)
	return fmt.Sprintf("%d-%d", time.Now().UnixNano(), n)
}

// MockHandler ...
type MockHandler struct {
	mu      sync.RWMutex
	Configs []model.MockConfig
	Path    string
	Log     zerolog.Logger
	Logs    *LogStore
}

// NewMockHandler ...
func NewMockHandler(path string) *MockHandler {
	cwd, _ := os.Getwd()
	log.Println("Current Working Directory:", cwd)
	log.Println("Loading configs from:", path)
	cfgs, err := config.LoadConfigs(path)
	if err != nil {
		log.Println("Load config error:", err)
	} else {
		log.Printf("Successfully loaded %d configs\n", len(cfgs))
	}
	if cfgs == nil {
		cfgs = []model.MockConfig{}
	}
	return &MockHandler{
		Configs: cfgs,
		Path:    path,
		Log:     zerolog.New(os.Stdout).With().Timestamp().Logger(),
		Logs:    NewLogStore(500),
	}
}

// Index ...
func (h *MockHandler) Index(c *fiber.Ctx) error {
	h.mu.RLock()
	configs := h.Configs
	h.mu.RUnlock()

	// Try to reload if empty (debugging purpose)
	if len(configs) == 0 {
		log.Println("Configs empty, attempting to reload from", h.Path)
		loaded, err := config.LoadConfigs(h.Path)
		if err == nil && len(loaded) > 0 {
			h.mu.Lock()
			h.Configs = loaded
			configs = h.Configs
			h.mu.Unlock()
			log.Println("Reloaded configs:", len(configs))
		}
	}

	c.Set("Content-Type", "text/html")
	return templates.Index(configs).Render(c.Context(), c.Response().BodyWriter())
}

// Save ...
func (h *MockHandler) Save(c *fiber.Ctx) error {
	// Log the raw body for debugging
	bodyBytes := c.Body()
	log.Printf("Received save request, body length: %d bytes", len(bodyBytes))
	log.Printf("Raw body preview (first 500 chars): %s", string(bodyBytes[:min(500, len(bodyBytes))]))

	var newCfgs []model.MockConfig
	if err := c.BodyParser(&newCfgs); err != nil {
		log.Printf("Error parsing config: %v", err)
		return c.Status(400).SendString("Invalid config format: " + err.Error())
	}

	log.Printf("Successfully parsed %d configurations", len(newCfgs))

	if err := config.SaveConfigs(h.Path, newCfgs); err != nil {
		log.Printf("Error saving configs: %v", err)
		return c.Status(500).SendString("Failed to save configs: " + err.Error())
	}

	h.mu.Lock()
	h.Configs = newCfgs
	h.mu.Unlock()
	log.Println("Configurations saved successfully")
	return c.SendString("Config saved")
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// Delete ...
func (h *MockHandler) Delete(c *fiber.Ctx) error {
	index, err := strconv.Atoi(c.Params("index"))
	if err != nil {
		return c.Status(400).SendString("Invalid index")
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	if index < 0 || index >= len(h.Configs) {
		return c.Status(400).SendString("Index out of range")
	}

	h.Configs = append(h.Configs[:index], h.Configs[index+1:]...)
	if err := config.SaveConfigs(h.Path, h.Configs); err != nil {
		return c.Status(500).SendString("Failed to save config")
	}
	return c.Redirect("/")
}

// BulkDelete ...
func (h *MockHandler) BulkDelete(c *fiber.Ctx) error {
	var body struct {
		Indices []int `json:"indices"`
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(400).SendString("Invalid request body")
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	newConfigs := make([]model.MockConfig, 0, len(h.Configs))
	indexMap := make(map[int]bool)
	for _, idx := range body.Indices {
		indexMap[idx] = true
	}

	for i, cfg := range h.Configs {
		if !indexMap[i] {
			newConfigs = append(newConfigs, cfg)
		}
	}

	h.Configs = newConfigs
	if err := config.SaveConfigs(h.Path, h.Configs); err != nil {
		return c.Status(500).SendString("Failed to save config")
	}

	return c.JSON(fiber.Map{"success": true, "message": fmt.Sprintf("Deleted %d configurations", len(body.Indices))})
}

// ImportOpenAPI imports OpenAPI/Swagger specification and generates mock configs
func (h *MockHandler) ImportOpenAPI(c *fiber.Ctx) error {
	// Get the uploaded file
	file, err := c.FormFile("file")
	if err != nil {
		log.Printf("Error getting file: %v", err)
		return c.Status(400).JSON(fiber.Map{"error": "No file uploaded"})
	}

	// Open the file
	fileContent, err := file.Open()
	if err != nil {
		log.Printf("Error opening file: %v", err)
		return c.Status(500).JSON(fiber.Map{"error": "Failed to read file"})
	}
	defer fileContent.Close()

	// Read file content
	data := make([]byte, file.Size)
	if _, err := fileContent.Read(data); err != nil {
		log.Printf("Error reading file content: %v", err)
		return c.Status(500).JSON(fiber.Map{"error": "Failed to read file content"})
	}

	// Determine if it's YAML or JSON based on file extension
	isYAML := strings.HasSuffix(strings.ToLower(file.Filename), ".yaml") ||
		strings.HasSuffix(strings.ToLower(file.Filename), ".yml")

	// Get merge option (default: true)
	merge := c.FormValue("merge") != "false"

	log.Printf("Importing OpenAPI file: %s (YAML: %v, Merge: %v)", file.Filename, isYAML, merge)

	// Try to parse as OpenAPI 3.x first
	configs, err := service.ParseOpenAPISpec(data, isYAML)
	if err != nil {
		// If OpenAPI 3.x fails, try Swagger 2.0
		log.Printf("OpenAPI 3.x parsing failed, trying Swagger 2.0: %v", err)
		configs, err = service.ParseSwagger2Spec(data, isYAML)
		if err != nil {
			log.Printf("Swagger 2.0 parsing also failed: %v", err)
			return c.Status(400).JSON(fiber.Map{"error": "Failed to parse specification: " + err.Error()})
		}
	}

	if len(configs) == 0 {
		return c.Status(400).JSON(fiber.Map{"error": "No endpoints found in specification"})
	}

	log.Printf("Successfully parsed %d endpoints from specification", len(configs))

	h.mu.Lock()
	defer h.mu.Unlock()

	if merge {
		// Merge with existing configs (append new ones)
		h.Configs = append(h.Configs, configs...)
	} else {
		// Replace all configs
		h.Configs = configs
	}

	// Save to file
	if err := config.SaveConfigs(h.Path, h.Configs); err != nil {
		log.Printf("Error saving configs: %v", err)
		return c.Status(500).JSON(fiber.Map{"error": "Failed to save configurations"})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"message": fmt.Sprintf("Successfully imported %d endpoints", len(configs)),
		"count":   len(configs),
	})
}

// Dynamic ...
func (h *MockHandler) Dynamic(c *fiber.Ctx) error {
	method := c.Method()
	path := c.Path()

	h.mu.RLock()
	configs := h.Configs
	h.mu.RUnlock()

	// Try to reload if empty (debugging purpose)
	if len(configs) == 0 {
		log.Println("Configs empty in Dynamic, attempting to reload from", h.Path)
		loaded, err := config.LoadConfigs(h.Path)
		if err == nil && len(loaded) > 0 {
			h.mu.Lock()
			h.Configs = loaded
			configs = h.Configs
			h.mu.Unlock()
			log.Println("Reloaded configs in Dynamic:", len(configs))
		}
	}

	log.Printf("[Dynamic] %s %s (Configs: %d)", method, path, len(configs))

	for _, cfg := range configs {
		if cfg.Method == method {
			if ok, params := pathMatch(cfg.Path, path); ok {
				// Build request context for rule evaluation
				ctx := buildRequestContext(c, params)

				// Check if config uses new conditional response format
				if len(cfg.Responses) > 0 {
					// Evaluate conditional responses in order
					for _, condResp := range cfg.Responses {
						if service.EvaluateRules(condResp.Rules, condResp.RuleOperator, ctx) {
							return sendResponse(c, condResp.Response, ctx)
						}
					}

					// If no conditional response matched, use default response
					if cfg.DefaultResponse != nil {
						return sendResponse(c, *cfg.DefaultResponse, ctx)
					}
				}

				// Backward compatibility: use old format
				// Validate headers
				headerMap := map[string]string{}
				for k := range cfg.RequestHeaders {
					val := c.Get(k)
					if val == "" {
						return c.Status(400).JSON(fiber.Map{"error": "missing header: " + k})
					}
					headerMap[k] = val
				}

				bodyMap := map[string]interface{}{}
				if (method == "POST" || method == "PUT") && cfg.RequestBody != nil {
					if err := c.BodyParser(&bodyMap); err != nil {
						return c.Status(400).JSON(fiber.Map{"error": "invalid body"})
					}
					for k := range cfg.RequestBody {
						if _, ok := bodyMap[k]; !ok {
							return c.Status(400).JSON(fiber.Map{"error": "missing body field: " + k})
						}
					}
				}
				queryMap := make(map[string]string)
				c.Request().URI().QueryArgs().VisitAll(func(k, v []byte) {
					queryMap[string(k)] = string(v)
				})

				rendered := service.RenderTemplateRecursive(cfg.ResponseBody, service.MapToStringMap(bodyMap), headerMap, queryMap, params)

				for k, v := range cfg.ResponseHeaders {
					c.Set(k, v)
				}
				if cfg.Timeout > 0 {
					time.Sleep(time.Duration(cfg.Timeout) * time.Millisecond)
				}

				return c.Status(cfg.StatusCode).JSON(rendered)
			}
		}
	}

	log.Printf("[Dynamic] No match found for %s %s", method, path)
	return c.Status(404).SendString(fmt.Sprintf("Mock not found for %s %s", method, path))
}

// buildRequestContext extracts request data into a RequestContext for rule evaluation
func buildRequestContext(c *fiber.Ctx, pathParams map[string]string) service.RequestContext {
	// Extract headers
	headerMap := make(map[string]string)
	c.Request().Header.VisitAll(func(key, value []byte) {
		headerMap[string(key)] = string(value)
	})

	// Extract body
	bodyMap := make(map[string]interface{})
	if c.Method() == "POST" || c.Method() == "PUT" || c.Method() == "PATCH" {
		_ = c.BodyParser(&bodyMap)
	}

	// Extract query params
	queryMap := make(map[string]string)
	c.Request().URI().QueryArgs().VisitAll(func(k, v []byte) {
		queryMap[string(k)] = string(v)
	})

	return service.RequestContext{
		Body:       service.MapToStringMap(bodyMap),
		Headers:    headerMap,
		Query:      queryMap,
		PathParams: pathParams,
	}
}

// sendResponse sends a response based on the Response configuration
func sendResponse(c *fiber.Ctx, resp model.Response, ctx service.RequestContext) error {
	// Render response body with template variables
	rendered := service.RenderTemplateRecursive(resp.Body, ctx.Body, ctx.Headers, ctx.Query, ctx.PathParams)

	// Set response headers
	for k, v := range resp.Headers {
		c.Set(k, v)
	}

	// Apply timeout if specified
	if resp.Timeout > 0 {
		time.Sleep(time.Duration(resp.Timeout) * time.Millisecond)
	}

	// Send response
	return c.Status(resp.StatusCode).JSON(rendered)
}

func pathMatch(cfgPath, reqPath string) (bool, map[string]string) {
	cfgParts := strings.Split(strings.Trim(cfgPath, "/"), "/")
	reqParts := strings.Split(strings.Trim(reqPath, "/"), "/")

	if len(cfgParts) != len(reqParts) {
		return false, nil
	}

	params := map[string]string{}
	for i := range cfgParts {
		if strings.HasPrefix(cfgParts[i], ":") {
			paramName := strings.TrimPrefix(cfgParts[i], ":")
			params[paramName] = reqParts[i]
		} else if cfgParts[i] != reqParts[i] {
			return false, nil
		}
	}
	return true, params
}

// RequestResponseLogger logs every dynamic request and stores it in the ring buffer.
func (h *MockHandler) RequestResponseLogger() fiber.Handler {
	return func(c *fiber.Ctx) error {
		start := time.Now()
		reqHeaders := c.GetReqHeaders()
		reqBody := c.Body()

		err := c.Next()

		duration := time.Since(start)
		resBody := c.Response().Body()
		statusCode := c.Response().StatusCode()

		// ----- zerolog stdout (as before) -----
		event := h.Log.Info().
			Str("METHOD", c.Method()).
			Str("URL", c.OriginalURL()).
			Any("HEAD", reqHeaders)
		if len(reqBody) > 0 {
			if json.Valid(reqBody) {
				event = event.RawJSON("BODY", reqBody)
			} else {
				event = event.Str("BODY", string(reqBody))
			}
		}
		if len(resBody) > 0 {
			if json.Valid(resBody) {
				event = event.RawJSON("RES", resBody)
			} else {
				event = event.Str("RES", string(resBody))
			}
		}
		event = event.Dur("DURATION", duration)
		if err != nil {
			event = event.Err(err)
		}
		event.Msg("API LOG")

		// ----- push to in-memory store -----
		// flatten request headers
		flatHeaders := make(map[string]string, len(reqHeaders))
		for k, vals := range reqHeaders {
			if len(vals) > 0 {
				flatHeaders[k] = vals[0]
			}
		}
		var reqBodyParsed interface{} = nil
		if len(reqBody) > 0 {
			if json.Valid(reqBody) {
				_ = json.Unmarshal(reqBody, &reqBodyParsed)
			} else {
				reqBodyParsed = string(reqBody)
			}
		}
		var resBodyParsed interface{} = nil
		if len(resBody) > 0 {
			if json.Valid(resBody) {
				_ = json.Unmarshal(resBody, &resBodyParsed)
			} else {
				resBodyParsed = string(resBody)
			}
		}
		errStr := ""
		if err != nil {
			errStr = err.Error()
		}
		matched := statusCode != 404
		h.Logs.Add(model.RequestLog{
			ID:         nextLogID(),
			Timestamp:  time.Now(),
			Method:     c.Method(),
			URL:        c.OriginalURL(),
			StatusCode: statusCode,
			DurationMs: float64(duration.Microseconds()) / 1000.0,
			ReqHeaders: flatHeaders,
			ReqBody:    reqBodyParsed,
			ResBody:    resBodyParsed,
			Matched:    matched,
			Error:      errStr,
		})
		return err
	}
}

// LogAPI returns all stored request logs as JSON.
func (h *MockHandler) LogAPI(c *fiber.Ctx) error {
	entries := h.Logs.GetAll()
	if entries == nil {
		entries = []model.RequestLog{}
	}
	return c.JSON(entries)
}

// ClearLogs removes all stored request logs.
func (h *MockHandler) ClearLogs(c *fiber.Ctx) error {
	h.Logs.Clear()
	return c.JSON(fiber.Map{"success": true, "message": "Logs cleared"})
}

// LogStream streams new log entries to the client via Server-Sent Events.
func (h *MockHandler) LogStream(c *fiber.Ctx) error {
	c.Set("Content-Type", "text/event-stream")
	c.Set("Cache-Control", "no-cache")
	c.Set("Connection", "keep-alive")
	c.Set("X-Accel-Buffering", "no")

	ch := h.Logs.Subscribe()

	c.Context().SetBodyStreamWriter(func(w *bufio.Writer) {
		defer h.Logs.Unsubscribe(ch)
		// send an initial ping so the client knows connection is alive
		_, _ = fmt.Fprintf(w, ": ping\n\n")
		_ = w.Flush()
		for entry := range ch {
			data, err := json.Marshal(entry)
			if err != nil {
				continue
			}
			_, werr := fmt.Fprintf(w, "data: %s\n\n", data)
			if werr != nil {
				return
			}
			if ferr := w.Flush(); ferr != nil {
				return
			}
		}
	})
	return nil
}

// parseJSONOrString is replaced by more efficient logic in Logger
