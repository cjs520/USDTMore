package main

import (
	"fmt"
	"os"
	"time"
	"strconv"
	"strings"
)

// 模拟GetExpireTime函数
func GetExpireTime() time.Duration {
	if ret := os.Getenv("EXPIRE_TIME"); ret != "" {
		sec, err := strconv.Atoi(ret)
		if err == nil && sec > 0 {
			return time.Duration(sec) * time.Second
		}
	}

	return 600 * time.Second // defaultExpireTime
}

func main() {
	// 测试不同的EXPIRE_TIME值
	testValues := []string{"", "600", "300", "1800"} // 空值、10分钟、5分钟、30分钟
	
	for _, val := range testValues {
		if val != "" {
			os.Setenv("EXPIRE_TIME", val)
		} else {
			os.Unsetenv("EXPIRE_TIME")
		}
		
		expireTime := GetExpireTime()
		expiredAt := time.Now().Add(expireTime)
		expire := int(time.Until(expiredAt).Seconds())
		
		fmt.Printf("EXPIRE_TIME=%s -> Duration=%v -> Seconds=%d\n", 
			strings.TrimSpace(val+"(default)"), expireTime, expire)
	}
}