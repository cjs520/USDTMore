package testutils

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
)

// MockHTTPServer 创建模拟HTTP服务器
type MockHTTPServer struct {
	Server   *httptest.Server
	Requests []MockRequest
}

// MockRequest 记录的请求信息
type MockRequest struct {
	Method  string
	URL     string
	Headers map[string]string
	Body    string
}

// NewMockHTTPServer 创建新的模拟HTTP服务器
func NewMockHTTPServer() *MockHTTPServer {
	mock := &MockHTTPServer{
		Requests: make([]MockRequest, 0),
	}

	mock.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 记录请求信息
		headers := make(map[string]string)
		for k, v := range r.Header {
			headers[k] = strings.Join(v, ",")
		}

		body := ""
		if r.Body != nil {
			buf := make([]byte, r.ContentLength)
			r.Body.Read(buf)
			body = string(buf)
		}

		mock.Requests = append(mock.Requests, MockRequest{
			Method:  r.Method,
			URL:     r.URL.String(),
			Headers: headers,
			Body:    body,
		})

		// 根据请求路径返回不同响应
		switch {
		case strings.Contains(r.URL.Path, "/notify"):
			// 模拟回调通知响应
			if strings.Contains(body, `"status":2`) { // 成功订单
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("ok"))
			} else {
				w.WriteHeader(http.StatusBadRequest)
				w.Write([]byte("failed"))
			}
		case strings.Contains(r.URL.Path, "/api/rate"):
			// 模拟汇率API响应
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"rate": 7.20,
			})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))

	return mock
}

// Close 关闭模拟服务器
func (m *MockHTTPServer) Close() {
	if m.Server != nil {
		m.Server.Close()
	}
}

// GetLastRequest 获取最后一个请求
func (m *MockHTTPServer) GetLastRequest() *MockRequest {
	if len(m.Requests) == 0 {
		return nil
	}
	return &m.Requests[len(m.Requests)-1]
}

// GetRequestCount 获取请求总数
func (m *MockHTTPServer) GetRequestCount() int {
	return len(m.Requests)
}

// ClearRequests 清空请求记录
func (m *MockHTTPServer) ClearRequests() {
	m.Requests = make([]MockRequest, 0)
}

// MockBlockchainAPI 创建模拟区块链API服务器
func NewMockBlockchainAPI() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		
		// 根据不同的查询参数返回不同的交易数据
		switch {
		case strings.Contains(r.URL.RawQuery, "tronscanapi.com"):
			// TronScan API 模拟响应
			response := map[string]interface{}{
				"total": 1,
				"token_transfers": []map[string]interface{}{
					{
						"transaction_id": "test_tx_hash_123",
						"to_address":     r.URL.Query().Get("relatedAddress"),
						"from_address":   "TTestSender123456789012345678901234",
						"quant":          100000000, // 100 USDT with 6 decimals
						"contractRet":    "SUCCESS",
						"block_ts":       1640995200000, // 2022-01-01
					},
				},
			}
			json.NewEncoder(w).Encode(response)
		case strings.Contains(r.URL.RawQuery, "etherscan"):
			// Etherscan类API模拟响应
			response := map[string]interface{}{
				"status": "1",
				"message": "OK",
				"result": []map[string]interface{}{
					{
						"hash":             "0xtest_eth_tx_hash",
						"to":               r.URL.Query().Get("address"),
						"from":             "0x1234567890123456789012345678901234567890",
						"value":            "100000000", // 100 USDT
						"tokenSymbol":      "USDT",
						"tokenDecimal":     "6",
						"contractAddress":  "0xdAC17F958D2ee523a2206206994597C13D831ec7",
						"timeStamp":        "1640995200",
					},
				},
			}
			json.NewEncoder(w).Encode(response)
		default:
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]string{"error": "not found"})
		}
	}))
}