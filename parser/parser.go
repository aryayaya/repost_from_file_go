package parser

import (
	"fmt"
	"strings"
)

// Request 表示解析后的 HTTP 请求
type Request struct {
	Method  string
	URL     string
	Headers map[string]string
	Body    string
}

// ParseFile 解析原始 HTTP 报文内容，返回 Request 结构
func ParseContent(content string) (*Request, error) {
	// 统一换行符
	content = strings.ReplaceAll(content, "\r\n", "\n")

	// 按空行分隔 head 和 body
	var headSection, body string
	if idx := strings.Index(content, "\n\n"); idx != -1 {
		headSection = content[:idx]
		body = strings.TrimSpace(content[idx+2:])
	} else {
		headSection = content
		body = ""
	}

	lines := strings.Split(headSection, "\n")
	if len(lines) == 0 {
		return nil, fmt.Errorf("报文为空")
	}

	// 解析请求行
	requestLine := strings.TrimSpace(lines[0])
	parts := strings.SplitN(requestLine, " ", 3)
	if len(parts) < 2 {
		return nil, fmt.Errorf("无效的请求行: %s", requestLine)
	}
	method := strings.ToUpper(parts[0])
	path := parts[1]

	// 解析 headers
	headers := make(map[string]string)
	host := ""
	for _, line := range lines[1:] {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if idx := strings.Index(line, ": "); idx != -1 {
			key := line[:idx]
			value := line[idx+2:]
			headers[key] = value
			if strings.ToLower(key) == "host" {
				host = value
			}
		}
	}

	if host == "" {
		return nil, fmt.Errorf("报文缺少 Host 头")
	}

	url := fmt.Sprintf("https://%s%s", host, path)

	return &Request{
		Method:  method,
		URL:     url,
		Headers: headers,
		Body:    body,
	}, nil
}
