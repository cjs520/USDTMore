package security

import (
	"USDTMore/app/config"
	"USDTMore/app/help"
	"USDTMore/app/log"
	"strings"
)

/*
在应用启动时验证安全配置
*/
func ValidateSecurityOnStartup() {
	log.Info("开始安全配置检查...")

	warnings := config.ValidateSecurityConfig()
	if len(warnings) > 0 {
		log.Warn("发现安全配置问题:")
		for _, warning := range warnings {
			log.Warn("- " + warning)
		}
	} else {
		log.Info("安全配置检查通过")
	}

	// 检查签名算法
	authToken := config.GetAuthToken()
	if authToken != "" && authToken != "123234" {
		log.Info("使用HMAC-SHA256签名算法")
	} else {
		log.Warn("AUTH_TOKEN配置不安全，建议设置强密钥")
	}

	// 检查HTTPS配置
	if strings.ToLower(help.GetEnv("ENVIRONMENT")) == "production" {
		if help.GetEnv("FORCE_HTTPS") != "true" {
			log.Warn("生产环境建议启用HTTPS")
		}
	}

	log.Info("安全配置检查完成")
}

/*
验证回调URL的安全性
*/
func ValidateCallbackURLSecurity(urls []string) []string {
	var unsafeURLs []string
	
	for _, url := range urls {
		if !help.IsValidCallbackURL(url) {
			unsafeURLs = append(unsafeURLs, url)
		}
	}
	
	return unsafeURLs
}