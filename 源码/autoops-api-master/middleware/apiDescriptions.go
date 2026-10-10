// API描述映射
// 自动生成，请勿手动修改
// 使用 sync_api_descriptions.py 生成

package middleware

import (
	"bufio"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
)

var (
	routerSummaryRegex = regexp.MustCompile(`(?i)@router\s+(\S+)\s+\[(\w+)\]`)
	directiveRegex     = regexp.MustCompile(`@[A-Za-z]+[^@]*`)
	swaggerOnce        sync.Once
	swaggerSummaryMap  map[string]string
	scanOnce           sync.Once
	scannedSummaryMap  map[string]string
)

// GetAPIDescription 根据 URL 路径和 HTTP 方法获取 API 的中文描述
func GetAPIDescription(url, method string) string {
	url = normalizeLookupPath(url)
	method = strings.ToLower(strings.TrimSpace(method))

	// 严格按 swagger 注解来源：summary > description
	// 1) docs/swagger.json（若存在并包含 paths）
	if desc, ok := lookupMappedDescription(getSwaggerSummaryMap(), method, url); ok {
		return desc
	}

	// 2) 代码注解扫描（@Router/@router + @Summary/@Description）
	if desc, ok := lookupMappedDescription(getScannedSummaryMap(), method, url); ok {
		return desc
	}

	// 3) 未在 swagger 定义，返回明确占位（避免误判为“新增/修改/删除”）
	return fallbackDescription(url, method)
}

// getDescriptionMap 返回 API 路径到描述的映射
func getDescriptionMap() map[string]string { return map[string]string{} }

// 兼容保留，避免其他代码引用报错；实际不再参与逻辑
func getMethodBasedDescriptionMap() map[string]string { return map[string]string{} }

func getScannedSummaryMap() map[string]string {
	scanOnce.Do(func() {
		scannedSummaryMap = scanRouterSummaries(resolveAPIRoot())
	})
	if scannedSummaryMap == nil {
		return map[string]string{}
	}
	return scannedSummaryMap
}

func getSwaggerSummaryMap() map[string]string {
	swaggerOnce.Do(func() {
		swaggerSummaryMap = loadSwaggerSummaries(resolveSwaggerJSONPath())
	})
	if swaggerSummaryMap == nil {
		return map[string]string{}
	}
	return swaggerSummaryMap
}

func resolveSwaggerJSONPath() string {
	if _, err := os.Stat(filepath.Join("docs", "swagger.json")); err == nil {
		return filepath.Join("docs", "swagger.json")
	}
	if _, currentFile, _, ok := runtime.Caller(0); ok {
		candidate := filepath.Join(filepath.Dir(currentFile), "..", "docs", "swagger.json")
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	return filepath.Join("docs", "swagger.json")
}

func loadSwaggerSummaries(path string) map[string]string {
	type swaggerOperation struct {
		Summary     string `json:"summary"`
		Description string `json:"description"`
	}
	type swaggerDoc struct {
		Paths map[string]map[string]swaggerOperation `json:"paths"`
	}

	result := make(map[string]string)
	data, err := os.ReadFile(path)
	if err != nil || len(data) == 0 {
		return result
	}

	var doc swaggerDoc
	if err := json.Unmarshal(data, &doc); err != nil {
		return result
	}

	for routePath, methods := range doc.Paths {
		normalizedPath := normalizeRoutePath(routePath)
		for method, op := range methods {
			desc := strings.TrimSpace(op.Summary)
			if desc == "" {
				desc = strings.TrimSpace(op.Description)
			}
			if desc == "" {
				continue
			}
			result[strings.ToLower(strings.TrimSpace(method))+":"+normalizedPath] = desc
		}
	}
	return result
}

func lookupMappedDescription(mapping map[string]string, method, url string) (string, bool) {
	if len(mapping) == 0 {
		return "", false
	}

	key := method + ":" + url
	if desc, ok := mapping[key]; ok {
		if trimmed := strings.TrimSpace(desc); trimmed != "" {
			return trimmed, true
		}
	}

	bestScore := -1
	bestDesc := ""
	for key, value := range mapping {
		routeMethod, routePath, ok := splitDescriptionKey(key)
		if !ok || routeMethod != method {
			continue
		}
		if matched, score := matchRoutePattern(routePath, url); matched && score > bestScore {
			trimmed := strings.TrimSpace(value)
			if trimmed == "" {
				continue
			}
			bestScore = score
			bestDesc = trimmed
		}
	}

	if bestScore >= 0 && bestDesc != "" {
		return bestDesc, true
	}
	return "", false
}

func splitDescriptionKey(key string) (string, string, bool) {
	sep := strings.Index(key, ":")
	if sep <= 0 || sep >= len(key)-1 {
		return "", "", false
	}
	method := strings.ToLower(strings.TrimSpace(key[:sep]))
	path := normalizeLookupPath(key[sep+1:])
	if method == "" || path == "" {
		return "", "", false
	}
	return method, path, true
}

func matchRoutePattern(routePath, requestPath string) (bool, int) {
	routePath = normalizeLookupPath(routePath)
	requestPath = normalizeLookupPath(requestPath)
	if routePath == requestPath {
		return true, 1 << 20 // 精确匹配优先级最高
	}

	routeParts := splitPath(routePath)
	requestParts := splitPath(requestPath)
	if len(routeParts) != len(requestParts) {
		return false, 0
	}

	score := 0
	for idx := range routeParts {
		routeSegment := routeParts[idx]
		requestSegment := requestParts[idx]
		if routeSegment == requestSegment {
			score += 2
			continue
		}
		if isWildcardSegment(routeSegment) {
			score++
			continue
		}
		return false, 0
	}

	return true, score
}

func splitPath(path string) []string {
	path = normalizeLookupPath(path)
	if path == "/" {
		return []string{}
	}
	return strings.Split(strings.TrimPrefix(path, "/"), "/")
}

func isWildcardSegment(segment string) bool {
	segment = strings.TrimSpace(segment)
	if segment == "" {
		return false
	}
	if strings.HasPrefix(segment, ":") && len(segment) > 1 {
		return true
	}
	if strings.HasPrefix(segment, "*") && len(segment) > 1 {
		return true
	}
	return strings.HasPrefix(segment, "{") && strings.HasSuffix(segment, "}") && len(segment) > 2
}

func normalizeLookupPath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return "/"
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	if len(path) > 1 {
		path = strings.TrimRight(path, "/")
	}
	if path == "" {
		return "/"
	}
	return path
}

func resolveAPIRoot() string {
	if _, err := os.Stat("api"); err == nil {
		return "api"
	}
	if _, currentFile, _, ok := runtime.Caller(0); ok {
		candidate := filepath.Join(filepath.Dir(currentFile), "..", "api")
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	return "api"
}

func scanRouterSummaries(root string) map[string]string {
	result := make(map[string]string)
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			name := d.Name()
			if name == "vendor" || strings.HasPrefix(name, ".") {
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Ext(path) != ".go" {
			return nil
		}

		parseSummaryFromFile(path, result)
		return nil
	})
	return result
}

func parseSummaryFromFile(path string, out map[string]string) {
	file, err := os.Open(path)
	if err != nil {
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	var block []string
	flush := func() {
		if len(block) == 0 {
			return
		}

		summary := ""
		description := ""
		var routes [][2]string // [path, method]
		for _, line := range block {
			segments := directiveRegex.FindAllString(line, -1)
			for _, seg := range segments {
				seg = strings.TrimSpace(seg)
				lower := strings.ToLower(seg)
				if strings.HasPrefix(lower, "@summary") {
					value := strings.TrimSpace(seg[len("@Summary"):])
					if value != "" {
						summary = value
					}
					continue
				}
				if strings.HasPrefix(lower, "@description") {
					value := strings.TrimSpace(seg[len("@Description"):])
					if value != "" {
						description = value
					}
					continue
				}
				if strings.HasPrefix(lower, "@router") {
					m := routerSummaryRegex.FindStringSubmatch(seg)
					if len(m) == 3 {
						routePath := normalizeRoutePath(m[1])
						method := strings.ToLower(strings.TrimSpace(m[2]))
						if routePath != "" && method != "" {
							routes = append(routes, [2]string{routePath, method})
						}
					}
				}
			}
		}

		if summary == "" {
			summary = description
		}
		if summary != "" {
			for _, route := range routes {
				out[route[1]+":"+route[0]] = summary
			}
		}

		block = nil
	}

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "//") {
			block = append(block, strings.TrimSpace(strings.TrimPrefix(line, "//")))
			continue
		}
		flush()
	}
	flush()
}

func normalizeRoutePath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	parts := strings.Split(path, "/")
	for i, part := range parts {
		if strings.HasPrefix(part, ":") && len(part) > 1 {
			parts[i] = "{" + part[1:] + "}"
			continue
		}
		if strings.HasPrefix(part, "*") && len(part) > 1 {
			parts[i] = "{" + part[1:] + "}"
		}
	}
	return strings.Join(parts, "/")
}

func fallbackDescription(url, method string) string {
	normalizedMethod := strings.ToUpper(strings.TrimSpace(method))
	if normalizedMethod == "" {
		normalizedMethod = "UNKNOWN"
	}
	normalizedURL := normalizeLookupPath(url)
	return "swagger未定义: " + normalizedMethod + " " + normalizedURL
}
