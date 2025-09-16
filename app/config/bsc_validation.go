package config

import (
	"fmt"
)

/*
验证BSC Web3 API配置是否完整
返回错误信息，如果配置正确则返回nil
*/
func ValidateBscWeb3Config() error {
	provider := GetBscWeb3Provider()
	
	switch provider {
	case WEB3_PROVIDER_MORALIS:
		if GetMoralisApiKey() == "" {
			return fmt.Errorf("BSC配置错误: 选择了MORALIS提供商但未设置MORALIS_API_KEY")
		}
		return nil
		
	case WEB3_PROVIDER_QUICKNODE:
		if GetQuickNodeApiKey() == "" {
			return fmt.Errorf("BSC配置错误: 选择了QUICKNODE提供商但未设置QUICKNODE_API_KEY")
		}
		if GetQuickNodeEndpoint() == "" {
			return fmt.Errorf("BSC配置错误: 选择了QUICKNODE提供商但未设置QUICKNODE_ENDPOINT")
		}
		return nil
		
	case WEB3_PROVIDER_ALCHEMY:
		if GetAlchemyApiKey() == "" {
			return fmt.Errorf("BSC配置错误: 选择了ALCHEMY提供商但未设置ALCHEMY_API_KEY")
		}
		return nil
		
	default:
		return fmt.Errorf("BSC配置错误: 不支持的Web3提供商 '%s'，请设置BSC_WEB3_PROVIDER为MORALIS、QUICKNODE或ALCHEMY", provider)
	}
}

/*
获取BSC配置建议
返回配置建议信息
*/
func GetBscConfigSuggestions() []string {
	var suggestions []string
	
	provider := GetBscWeb3Provider()
	
	suggestions = append(suggestions, "BSC Web3 API提供商配置建议:")
	suggestions = append(suggestions, "")
	
	switch provider {
	case WEB3_PROVIDER_MORALIS:
		suggestions = append(suggestions, "✅ 当前使用: MORALIS (推荐)")
		if GetMoralisApiKey() == "" {
			suggestions = append(suggestions, "❌ 缺少配置: MORALIS_API_KEY")
			suggestions = append(suggestions, "   获取地址: https://moralis.io")
		} else {
			suggestions = append(suggestions, "✅ MORALIS_API_KEY 已配置")
		}
		
	case WEB3_PROVIDER_QUICKNODE:
		suggestions = append(suggestions, "✅ 当前使用: QUICKNODE")
		if GetQuickNodeApiKey() == "" {
			suggestions = append(suggestions, "❌ 缺少配置: QUICKNODE_API_KEY")
		} else {
			suggestions = append(suggestions, "✅ QUICKNODE_API_KEY 已配置")
		}
		if GetQuickNodeEndpoint() == "" {
			suggestions = append(suggestions, "❌ 缺少配置: QUICKNODE_ENDPOINT")
			suggestions = append(suggestions, "   获取地址: https://quicknode.com")
		} else {
			suggestions = append(suggestions, "✅ QUICKNODE_ENDPOINT 已配置")
		}
		
	case WEB3_PROVIDER_ALCHEMY:
		suggestions = append(suggestions, "✅ 当前使用: ALCHEMY")
		if GetAlchemyApiKey() == "" {
			suggestions = append(suggestions, "❌ 缺少配置: ALCHEMY_API_KEY")
			suggestions = append(suggestions, "   获取地址: https://alchemy.com")
		} else {
			suggestions = append(suggestions, "✅ ALCHEMY_API_KEY 已配置")
		}
		
	default:
		suggestions = append(suggestions, fmt.Sprintf("❌ 不支持的提供商: %s", provider))
		suggestions = append(suggestions, "   请设置 BSC_WEB3_PROVIDER 为以下值之一:")
		suggestions = append(suggestions, "   - MORALIS (推荐，免费额度)")
		suggestions = append(suggestions, "   - QUICKNODE (付费)")
		suggestions = append(suggestions, "   - ALCHEMY (免费额度)")
	}
	
	suggestions = append(suggestions, "")
	suggestions = append(suggestions, "注意: BSC已不再支持传统的BSCScan API")
	
	return suggestions
}