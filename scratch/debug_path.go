package main

import (
	"fmt"
	"strings"
)

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

func main() {
	tests := []struct {
		cfg string
		req string
	}{
		{"/api/test", "/api/test"},
		{"/api/test/", "/api/test"},
		{"/api/test", "/api/test/"},
		{"/", "/"},
		{"/api/test/:id", "/api/test/123"},
		{"", "/"},
		{"/", ""},
	}

	for _, t := range tests {
		ok, _ := pathMatch(t.cfg, t.req)
		fmt.Printf("cfg: %q, req: %q => %v\n", t.cfg, t.req, ok)
	}
}
