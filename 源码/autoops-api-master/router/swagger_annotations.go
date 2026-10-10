package router

import (
	"bufio"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
)

var (
	swaggerAnnotationSegmentRegex = regexp.MustCompile(`@[A-Za-z]+[^@]*`)
	swaggerParamRegex             = regexp.MustCompile(`^@Param\s+(\S+)\s+(\S+)\s+(\S+)\s+(\S+)\s*(?:"([^"]*)")?.*$`)
	swaggerSuccessRegex           = regexp.MustCompile(`^@Success\s+(\d+)\s+\{(\w+)\}\s+(\S+)\s*(?:"([^"]*)")?.*$`)

	swaggerAnnotationsOnce sync.Once
	swaggerAnnotationOps   map[string]map[string]gin.H
)

type parsedSwaggerOperation struct {
	path        string
	method      string
	summary     string
	description string
	tags        []string
	parameters  []gin.H
	responses   map[string]gin.H
	security    []gin.H
}

func mergeSwaggerAnnotationOperations(paths map[string]gin.H) {
	ops := loadSwaggerAnnotationOperations()
	for path, methods := range ops {
		if _, exists := paths[path]; !exists {
			paths[path] = gin.H{}
		}
		for method, operation := range methods {
			paths[path][method] = operation
		}
	}
}

func loadSwaggerAnnotationOperations() map[string]map[string]gin.H {
	swaggerAnnotationsOnce.Do(func() {
		swaggerAnnotationOps = parseSwaggerAnnotations(resolveSwaggerScanRoot())
	})
	if swaggerAnnotationOps == nil {
		return map[string]map[string]gin.H{}
	}
	return swaggerAnnotationOps
}

func resolveSwaggerScanRoot() string {
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

func parseSwaggerAnnotations(root string) map[string]map[string]gin.H {
	operations := make(map[string]map[string]gin.H)

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

		parseSwaggerFile(path, operations)
		return nil
	})

	return operations
}

func parseSwaggerFile(path string, operations map[string]map[string]gin.H) {
	file, err := os.Open(path)
	if err != nil {
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	var commentBlock []string
	flushBlock := func() {
		if len(commentBlock) == 0 {
			return
		}
		if op := parseSwaggerCommentBlock(commentBlock); op != nil {
			if _, exists := operations[op.path]; !exists {
				operations[op.path] = make(map[string]gin.H)
			}
			operations[op.path][op.method] = op.toMap()
		}
		commentBlock = nil
	}

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "//") {
			commentBlock = append(commentBlock, strings.TrimSpace(strings.TrimPrefix(line, "//")))
			continue
		}
		flushBlock()
	}
	flushBlock()
}

func parseSwaggerCommentBlock(lines []string) *parsedSwaggerOperation {
	op := &parsedSwaggerOperation{
		responses: make(map[string]gin.H),
	}
	hasDirective := false

	for _, line := range lines {
		segments := swaggerAnnotationSegmentRegex.FindAllString(line, -1)
		for _, segment := range segments {
			segment = strings.TrimSpace(segment)
			if segment == "" {
				continue
			}
			hasDirective = true
			applySwaggerAnnotation(op, segment)
		}
	}

	if !hasDirective || op.path == "" || op.method == "" {
		return nil
	}

	if len(op.responses) == 0 {
		op.responses["200"] = gin.H{"description": "OK"}
	}

	return op
}

func applySwaggerAnnotation(op *parsedSwaggerOperation, annotation string) {
	switch {
	case strings.HasPrefix(annotation, "@Summary"):
		op.summary = strings.TrimSpace(strings.TrimPrefix(annotation, "@Summary"))
	case strings.HasPrefix(annotation, "@Description"):
		op.description = strings.TrimSpace(strings.TrimPrefix(annotation, "@Description"))
	case strings.HasPrefix(annotation, "@Tags"):
		rawTags := strings.Fields(strings.TrimSpace(strings.TrimPrefix(annotation, "@Tags")))
		for _, tag := range rawTags {
			if tag == "" {
				continue
			}
			exists := false
			for _, existing := range op.tags {
				if existing == tag {
					exists = true
					break
				}
			}
			if !exists {
				op.tags = append(op.tags, tag)
			}
		}
	case strings.HasPrefix(annotation, "@Router"):
		parseRouterAnnotation(op, annotation)
	case strings.HasPrefix(annotation, "@Param"):
		parseParamAnnotation(op, annotation)
	case strings.HasPrefix(annotation, "@Success"):
		parseSuccessAnnotation(op, annotation)
	case strings.HasPrefix(annotation, "@Security"):
		parseSecurityAnnotation(op, annotation)
	}
}

func parseRouterAnnotation(op *parsedSwaggerOperation, annotation string) {
	payload := strings.TrimSpace(strings.TrimPrefix(annotation, "@Router"))
	parts := strings.Fields(payload)
	if len(parts) < 2 {
		return
	}

	path := parts[0]
	methodToken := strings.TrimSpace(parts[1])
	methodToken = strings.TrimPrefix(methodToken, "[")
	methodToken = strings.TrimSuffix(methodToken, "]")
	if methodToken == "" {
		return
	}

	op.path = toSwaggerPath(path)
	op.method = strings.ToLower(methodToken)
}

func parseParamAnnotation(op *parsedSwaggerOperation, annotation string) {
	matches := swaggerParamRegex.FindStringSubmatch(annotation)
	if len(matches) < 5 {
		return
	}

	name := matches[1]
	inType := strings.ToLower(matches[2])
	dataType := matches[3]
	required := strings.EqualFold(matches[4], "true")
	description := ""
	if len(matches) > 5 {
		description = strings.TrimSpace(matches[5])
	}
	if inType == "path" {
		required = true
	}

	param := gin.H{
		"name":     name,
		"in":       inType,
		"required": required,
	}
	if description != "" {
		param["description"] = description
	}

	switch inType {
	case "body":
		param["schema"] = schemaFromType(dataType)
	default:
		if strings.HasPrefix(dataType, "[]") {
			param["type"] = "array"
			param["items"] = schemaFromType(strings.TrimPrefix(dataType, "[]"))
		} else {
			param["type"] = primitiveType(dataType)
		}
	}

	op.parameters = append(op.parameters, param)
}

func parseSuccessAnnotation(op *parsedSwaggerOperation, annotation string) {
	matches := swaggerSuccessRegex.FindStringSubmatch(annotation)
	if len(matches) < 4 {
		return
	}

	code := matches[1]
	schemaKind := strings.ToLower(matches[2])
	schemaType := cleanSchemaType(matches[3])
	description := ""
	if len(matches) > 4 {
		description = strings.TrimSpace(matches[4])
	}
	if description == "" {
		if statusCode, err := strconv.Atoi(code); err == nil {
			description = defaultStatusText(statusCode)
		}
		if description == "" {
			description = "OK"
		}
	}

	response := gin.H{
		"description": description,
	}

	switch schemaKind {
	case "array":
		response["schema"] = gin.H{
			"type":  "array",
			"items": schemaFromType(schemaType),
		}
	case "object":
		response["schema"] = schemaFromType(schemaType)
	default:
		response["schema"] = gin.H{
			"type": primitiveType(schemaKind),
		}
	}

	op.responses[code] = response
}

func parseSecurityAnnotation(op *parsedSwaggerOperation, annotation string) {
	payload := strings.TrimSpace(strings.TrimPrefix(annotation, "@Security"))
	parts := strings.Fields(payload)
	if len(parts) == 0 {
		return
	}

	securityName := strings.TrimSpace(parts[0])
	if securityName == "" {
		return
	}

	op.security = append(op.security, gin.H{
		securityName: []string{},
	})
}

func (op *parsedSwaggerOperation) toMap() gin.H {
	operation := gin.H{
		"responses": op.responses,
	}
	if op.summary != "" {
		operation["summary"] = op.summary
	}
	if op.description != "" {
		operation["description"] = op.description
	}
	if len(op.tags) > 0 {
		operation["tags"] = op.tags
	}
	if len(op.parameters) > 0 {
		operation["parameters"] = op.parameters
	}
	if len(op.security) > 0 {
		operation["security"] = op.security
	}
	return operation
}

func schemaFromType(dataType string) gin.H {
	dataType = strings.TrimSpace(dataType)
	dataType = cleanSchemaType(dataType)
	if dataType == "" {
		return gin.H{"type": "object"}
	}

	if strings.HasPrefix(dataType, "[]") {
		return gin.H{
			"type":  "array",
			"items": schemaFromType(strings.TrimPrefix(dataType, "[]")),
		}
	}

	lower := strings.ToLower(dataType)
	switch lower {
	case "string":
		return gin.H{"type": "string"}
	case "bool", "boolean":
		return gin.H{"type": "boolean"}
	case "int", "int8", "int16", "int32", "int64", "uint", "uint8", "uint16", "uint32", "uint64":
		return gin.H{"type": "integer"}
	case "float32", "float64", "number":
		return gin.H{"type": "number"}
	case "file":
		return gin.H{"type": "file"}
	case "object":
		return gin.H{"type": "object"}
	}

	return gin.H{"type": "object"}
}

func primitiveType(dataType string) string {
	dataType = strings.TrimSpace(strings.TrimPrefix(dataType, "[]"))
	switch strings.ToLower(dataType) {
	case "bool", "boolean":
		return "boolean"
	case "int", "int8", "int16", "int32", "int64", "uint", "uint8", "uint16", "uint32", "uint64":
		return "integer"
	case "float32", "float64", "number":
		return "number"
	case "file":
		return "file"
	case "object":
		return "object"
	default:
		return "string"
	}
}

func cleanSchemaType(schemaType string) string {
	schemaType = strings.TrimSpace(schemaType)
	if idx := strings.Index(schemaType, "{"); idx > 0 {
		schemaType = schemaType[:idx]
	}
	return strings.TrimSpace(schemaType)
}

func defaultStatusText(code int) string {
	switch code {
	case 200:
		return "OK"
	case 201:
		return "Created"
	case 202:
		return "Accepted"
	case 204:
		return "No Content"
	case 400:
		return "Bad Request"
	case 401:
		return "Unauthorized"
	case 403:
		return "Forbidden"
	case 404:
		return "Not Found"
	case 500:
		return "Internal Server Error"
	default:
		return ""
	}
}
