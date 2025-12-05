package middleware

import (
	"AgentEarth-Mgr/admin/internal/types"
	"bytes"
	"encoding/json"
	"net/http"

	"github.com/zeromicro/go-zero/core/logx"
)

// ErrorHandlerMiddleware 错误拦截中间件，将错误响应转换为 BaseResp 格式
func ErrorHandlerMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// 创建一个 ResponseWriter 包装器来捕获响应
		writer := &responseWriter{
			ResponseWriter: w,
			body:           &bytes.Buffer{},
			statusCode:     http.StatusOK,
		}

		// 执行下一个 handler
		next(writer, r)

		// 如果状态码表示错误（4xx 或 5xx）
		if writer.statusCode >= 400 {
			// 读取响应体
			bodyBytes := writer.body.Bytes()
			
			// 创建错误响应
			var baseResp types.BaseResp
			
			// 尝试解析响应体，可能是 go-zero 的错误格式或其他格式
			var errorResp map[string]interface{}
			if err := json.Unmarshal(bodyBytes, &errorResp); err == nil {
				// 如果是 JSON 格式，尝试提取错误信息
				if msg, ok := errorResp["message"].(string); ok {
					baseResp.Message = msg
				} else if msg, ok := errorResp["error"].(string); ok {
					baseResp.Message = msg
				} else {
					baseResp.Message = string(bodyBytes)
				}
			} else {
				// 如果不是 JSON 格式，直接使用响应体作为错误信息
				baseResp.Message = string(bodyBytes)
			}

			// 设置错误响应格式
			baseResp.Code = 1000
			if baseResp.Message == "" {
				baseResp.Message = http.StatusText(writer.statusCode)
			}
			baseResp.Data = nil

			// 写入响应
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(writer.statusCode)
			if err := json.NewEncoder(w).Encode(baseResp); err != nil {
				logx.Errorf("Failed to encode error response: %v", err)
			}
		} else {
			// 正常响应，直接写入原始内容
			w.WriteHeader(writer.statusCode)
			_, _ = writer.body.WriteTo(w)
		}
	}
}

// responseWriter 包装 http.ResponseWriter 以捕获响应内容
type responseWriter struct {
	http.ResponseWriter
	body       *bytes.Buffer
	statusCode int
	wroteHeader bool
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	if !rw.wroteHeader {
		rw.WriteHeader(http.StatusOK)
	}
	return rw.body.Write(b)
}

func (rw *responseWriter) WriteHeader(statusCode int) {
	if !rw.wroteHeader {
		rw.statusCode = statusCode
		rw.wroteHeader = true
	}
}

