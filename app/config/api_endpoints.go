package config

import (
	"fmt"
	"strings"
)

// API端点配置，支持备用端点以提高可靠性
type APIEndpointConfig struct {
	Primary   string   // 主要端点
	Fallbacks []string // 备用端点
	ChainID   string   // 链ID
}

// 获取EVM兼容链的API端点配置
func GetEVMChainAPIEndpoints(chain string) *APIEndpointConfig {
	switch strings.ToUpper(chain) {
	case "POLY", "POLYGON":
		return &APIEndpointConfig{
			Primary: "https://api.etherscan.io/v2/api",
			Fallbacks: []string{
				"https://api.polygonscan.com/api", // Polygon专用API作为备用
			},
			ChainID: "137",
		}
	case "OP", "OPTIMISM":
		return &APIEndpointConfig{
			Primary: "https://api.etherscan.io/v2/api",
			Fallbacks: []string{
				"https://api-optimistic.etherscan.io/api", // Optimism专用API作为备用
			},
			ChainID: "10",
		}
	case "BSC":
		// BSC不使用Etherscan API，使用Web3提供商
		return &APIEndpointConfig{
			Primary:   "", // BSC使用专用Web3 API
			Fallbacks: []string{},
			ChainID:   "56",
		}
	case "ARB", "ARBITRUM":
		return &APIEndpointConfig{
			Primary: "https://api.etherscan.io/v2/api",
			Fallbacks: []string{
				"https://api.arbiscan.io/api", // Arbitrum专用API作为备用
			},
			ChainID: "42161",
		}
	case "XLAYER":
		return &APIEndpointConfig{
			Primary:   "https://api.etherscan.io/v2/api",
			Fallbacks: []string{
				// X-Layer可能需要专用API端点
			},
			ChainID: "196",
		}
	default:
		return &APIEndpointConfig{
			Primary:   "https://api.etherscan.io/v2/api",
			Fallbacks: []string{},
			ChainID:   "1", // 默认以太坊主网
		}
	}
}

// 获取所有可用的API端点（主要+备用）
func (config *APIEndpointConfig) GetAllEndpoints() []string {
	endpoints := []string{}
	if config.Primary != "" {
		endpoints = append(endpoints, config.Primary)
	}
	endpoints = append(endpoints, config.Fallbacks...)
	return endpoints
}

// 构建API查询URL
func (config *APIEndpointConfig) BuildQueryURL(endpoint, module, action, address, apiKey string, extraParams map[string]string) string {
	// 检查是否是Etherscan V2 API
	isV2API := strings.Contains(endpoint, "/v2/api")

	var params []string

	if isV2API {
		// Etherscan V2 API格式
		params = append(params, fmt.Sprintf("chainid=%s", config.ChainID))
	}

	// 基本参数
	params = append(params, fmt.Sprintf("module=%s", module))
	params = append(params, fmt.Sprintf("action=%s", action))
	params = append(params, fmt.Sprintf("address=%s", address))

	// 额外参数
	for key, value := range extraParams {
		params = append(params, fmt.Sprintf("%s=%s", key, value))
	}

	// API Key
	params = append(params, fmt.Sprintf("apikey=%s", apiKey))

	return fmt.Sprintf("%s?%s", endpoint, strings.Join(params, "&"))
}

// 获取推荐的超时时间（秒）
func (config *APIEndpointConfig) GetRecommendedTimeout() int {
	// 根据不同的API端点返回推荐的超时时间
	if strings.Contains(config.Primary, "etherscan.io/v2") {
		return 150 // Etherscan V2 API需要更长的超时时间
	}
	return 90 // 其他API的默认超时时间
}

// 检查API端点是否支持特定功能
func (config *APIEndpointConfig) SupportsTokenBalance() bool {
	// Etherscan V2 API不直接支持tokenbalance查询
	return !strings.Contains(config.Primary, "/v2/api")
}

// 获取API端点的显示名称
func (config *APIEndpointConfig) GetDisplayName(endpoint string) string {
	if strings.Contains(endpoint, "etherscan.io/v2") {
		return "Etherscan V2 API"
	} else if strings.Contains(endpoint, "polygonscan.com") {
		return "PolygonScan API"
	} else if strings.Contains(endpoint, "optimistic.etherscan.io") {
		return "Optimism Etherscan API"
	} else if strings.Contains(endpoint, "arbiscan.io") {
		return "Arbiscan API"
	}
	return "Unknown API"
}
