package service

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/brianvoe/gofakeit/v6"
)

var fakerRegex = regexp.MustCompile(`\{\{faker\.([a-zA-Z0-9_]+)\}\}`)
var templateRegex = regexp.MustCompile(`\{\{(body|header|query|path)\.([a-zA-Z0-9_\.\-]+)\}\}`)

var fakerMap = map[string]func() string{
	"name":          gofakeit.Name,
	"first_name":    gofakeit.FirstName,
	"last_name":     gofakeit.LastName,
	"email":         gofakeit.Email,
	"phone":         gofakeit.Phone,
	"company":       gofakeit.Company,
	"address":       func() string { return gofakeit.Address().Address },
	"city":          gofakeit.City,
	"country":       gofakeit.Country,
	"uuid":          gofakeit.UUID,
	"word":          gofakeit.Word,
	"sentence":      func() string { return gofakeit.Sentence(5) },
	"paragraph":     func() string { return gofakeit.Paragraph(1, 2, 5, " ") },
	"ipv4":          gofakeit.IPv4Address,
	"user_agent":    gofakeit.UserAgent,
	"number":        func() string { return fmt.Sprintf("%d", gofakeit.Number(1, 1000)) },
	"boolean":       func() string { return fmt.Sprintf("%t", gofakeit.Bool()) },
	"date":          func() string { return gofakeit.Date().Format("2006-01-02") },
	"time":          func() string { return gofakeit.Date().Format("15:04:05") },
	"hex_color":     gofakeit.HexColor,
	"currency_code": func() string { return gofakeit.Currency().Short },
}

// RenderTemplateRecursive ...
func RenderTemplateRecursive(data interface{}, bodyMap, headerMap, queryMap, pathMap map[string]string) interface{} {
	switch v := data.(type) {
	case string:
		v = templateRegex.ReplaceAllStringFunc(v, func(m string) string {
			match := templateRegex.FindStringSubmatch(m)
			if len(match) > 2 {
				source := match[1]
				key := match[2]
				switch source {
				case "body":
					if val, ok := bodyMap[key]; ok {
						return val
					}
				case "header":
					if val, ok := headerMap[strings.ToLower(key)]; ok {
						return val
					}
				case "query":
					if val, ok := queryMap[key]; ok {
						return val
					}
				case "path":
					if val, ok := pathMap[key]; ok {
						return val
					}
				}
			}
			return m
		})

		// Replace faker placeholders
		v = fakerRegex.ReplaceAllStringFunc(v, func(m string) string {
			match := fakerRegex.FindStringSubmatch(m)
			if len(match) > 1 {
				key := strings.ToLower(match[1])
				if fn, ok := fakerMap[key]; ok {
					return fn()
				}
			}
			return m
		})

		return v
	case map[string]interface{}:
		result := make(map[string]interface{})
		for key, val := range v {
			result[key] = RenderTemplateRecursive(val, bodyMap, headerMap, queryMap, pathMap)
		}
		return result
	case []interface{}:
		result := make([]interface{}, len(v))
		for i, val := range v {
			result[i] = RenderTemplateRecursive(val, bodyMap, headerMap, queryMap, pathMap)
		}
		return result
	}
	return data
}

// MapToStringMap ...
func MapToStringMap(input map[string]interface{}) map[string]string {
	output := make(map[string]string)
	for k, v := range input {
		output[k] = fmt.Sprintf("%v", v)
	}
	return output
}

// FlattenMap recursively flattens a nested map into dot-notation keys.
// e.g. {"merchant": {"name": "Test"}} becomes {"merchant.name": "Test"}
func FlattenMap(input map[string]interface{}, prefix string) map[string]string {
	output := make(map[string]string)
	for k, v := range input {
		fullKey := k
		if prefix != "" {
			fullKey = prefix + "." + k
		}
		switch val := v.(type) {
		case map[string]interface{}:
			for nk, nv := range FlattenMap(val, fullKey) {
				output[nk] = nv
			}
		case []interface{}:
			for i, item := range val {
				arrayKey := fmt.Sprintf("%s.%d", fullKey, i)
				switch arrVal := item.(type) {
				case map[string]interface{}:
					for nk, nv := range FlattenMap(arrVal, arrayKey) {
						output[nk] = nv
					}
				default:
					output[arrayKey] = fmt.Sprintf("%v", arrVal)
				}
			}
			output[fullKey] = fmt.Sprintf("%v", v)
		default:
			output[fullKey] = fmt.Sprintf("%v", v)
		}
	}
	return output
}
