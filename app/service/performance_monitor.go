package service

import (
	"USDTMore/app/config"
	"USDTMore/app/model"
	"context"
	"database/sql"
	"fmt"
	"log"
	"sync"
	"time"

	"gorm.io/gorm"
)

// PerformanceMonitor 性能监控服务
type PerformanceMonitor struct {
	db              *gorm.DB
	alertChannel    chan Alert
	metrics         *PerformanceMetrics
	mu              sync.RWMutex
	isRunning       bool
	stopChannel     chan bool
	alertThresholds AlertThresholds
}

// PerformanceMetrics 性能指标
type PerformanceMetrics struct {
	// 数据库连接池指标
	OpenConnections    int     `json:"open_connections"`
	InUseConnections   int     `json:"in_use_connections"`
	IdleConnections    int     `json:"idle_connections"`
	ConnectionUsageRate float64 `json:"connection_usage_rate"`
	WaitCount          int64   `json:"wait_count"`
	AvgWaitTime        time.Duration `json:"avg_wait_time"`

	// 查询性能指标
	SlowQueryCount     int64         `json:"slow_query_count"`
	AvgQueryTime       time.Duration `json:"avg_query_time"`
	QueryErrorRate     float64       `json:"query_error_rate"`
	DeadlockCount      int64         `json:"deadlock_count"`

	// 业务指标
	OrderCreateRate    float64 `json:"order_create_rate"`    // 每秒创建订单数
	OrderSuccessRate   float64 `json:"order_success_rate"`   // 订单成功率
	NotifyFailureRate  float64 `json:"notify_failure_rate"`  // 通知失败率
	ExpiredOrderCount  int64   `json:"expired_order_count"`  // 过期订单数量

	// 系统指标
	LastUpdated        time.Time `json:"last_updated"`
	UptimeSeconds      int64     `json:"uptime_seconds"`
}

// Alert 告警
type Alert struct {
	Type        string                 `json:"type"`        // WARNING, CRITICAL, INFO
	Message     string                 `json:"message"`
	Component   string                 `json:"component"`   // DATABASE, APPLICATION, BUSINESS
	Metric      string                 `json:"metric"`
	Value       interface{}            `json:"value"`
	Threshold   interface{}            `json:"threshold"`
	Timestamp   time.Time              `json:"timestamp"`
	Details     map[string]interface{} `json:"details"`
}

// AlertThresholds 告警阈值配置
type AlertThresholds struct {
	ConnectionUsageWarning    float64       `json:"connection_usage_warning"`     // 80%
	ConnectionUsageCritical   float64       `json:"connection_usage_critical"`    // 95%
	SlowQueryThreshold        time.Duration `json:"slow_query_threshold"`         // 1s
	QueryErrorRateWarning     float64       `json:"query_error_rate_warning"`     // 5%
	QueryErrorRateCritical    float64       `json:"query_error_rate_critical"`    // 10%
	OrderSuccessRateWarning   float64       `json:"order_success_rate_warning"`   // 90%
	OrderSuccessRateCritical  float64       `json:"order_success_rate_critical"`  // 85%
	NotifyFailureRateWarning  float64       `json:"notify_failure_rate_warning"`  // 10%
	NotifyFailureRateCritical float64       `json:"notify_failure_rate_critical"` // 20%
	WaitTimeWarning           time.Duration `json:"wait_time_warning"`            // 100ms
	WaitTimeCritical          time.Duration `json:"wait_time_critical"`           // 500ms
}

// NewPerformanceMonitor 创建性能监控器
func NewPerformanceMonitor(db *gorm.DB) *PerformanceMonitor {
	return &PerformanceMonitor{
		db:           db,
		alertChannel: make(chan Alert, 100),
		metrics:      &PerformanceMetrics{},
		stopChannel:  make(chan bool),
		alertThresholds: AlertThresholds{
			ConnectionUsageWarning:    80.0,
			ConnectionUsageCritical:   95.0,
			SlowQueryThreshold:        config.GetDbSlowQueryThreshold(),
			QueryErrorRateWarning:     5.0,
			QueryErrorRateCritical:    10.0,
			OrderSuccessRateWarning:   90.0,
			OrderSuccessRateCritical:  85.0,
			NotifyFailureRateWarning:  10.0,
			NotifyFailureRateCritical: 20.0,
			WaitTimeWarning:           100 * time.Millisecond,
			WaitTimeCritical:          500 * time.Millisecond,
		},
	}
}

// Start 启动性能监控
func (pm *PerformanceMonitor) Start(ctx context.Context) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	if pm.isRunning {
		return fmt.Errorf("performance monitor is already running")
	}

	pm.isRunning = true
	
	// 启动指标收集协程
	go pm.collectMetrics(ctx)
	
	// 启动告警处理协程
	go pm.handleAlerts(ctx)

	log.Println("Performance monitor started")
	return nil
}

// Stop 停止性能监控
func (pm *PerformanceMonitor) Stop() error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	if !pm.isRunning {
		return nil
	}

	pm.isRunning = false
	close(pm.stopChannel)
	close(pm.alertChannel)

	log.Println("Performance monitor stopped")
	return nil
}

// collectMetrics 收集性能指标
func (pm *PerformanceMonitor) collectMetrics(ctx context.Context) {
	interval := config.GetDbMonitoringInterval()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	startTime := time.Now()

	for {
		select {
		case <-ctx.Done():
			return
		case <-pm.stopChannel:
			return
		case <-ticker.C:
			pm.collectAndAnalyzeMetrics(ctx, startTime)
		}
	}
}

// collectAndAnalyzeMetrics 收集并分析指标
func (pm *PerformanceMonitor) collectAndAnalyzeMetrics(ctx context.Context, startTime time.Time) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	// 收集数据库连接池指标
	pm.collectConnectionPoolMetrics()

	// 收集查询性能指标
	pm.collectQueryMetrics(ctx)

	// 收集业务指标
	pm.collectBusinessMetrics(ctx)

	// 更新系统指标
	pm.metrics.LastUpdated = time.Now()
	pm.metrics.UptimeSeconds = int64(time.Since(startTime).Seconds())

	// 分析指标并生成告警
	pm.analyzeMetricsAndAlert()
}

// collectConnectionPoolMetrics 收集连接池指标
func (pm *PerformanceMonitor) collectConnectionPoolMetrics() {
	sqlDB, err := pm.db.DB()
	if err != nil {
		log.Printf("Failed to get sql.DB: %v", err)
		return
	}

	stats := sqlDB.Stats()
	pm.metrics.OpenConnections = stats.OpenConnections
	pm.metrics.InUseConnections = stats.InUse
	pm.metrics.IdleConnections = stats.Idle
	pm.metrics.WaitCount = stats.WaitCount

	// 计算连接使用率
	if stats.MaxOpenConnections > 0 {
		pm.metrics.ConnectionUsageRate = float64(stats.InUse) / float64(stats.MaxOpenConnections) * 100
	}

	// 计算平均等待时间
	if stats.WaitCount > 0 {
		pm.metrics.AvgWaitTime = stats.WaitDuration / time.Duration(stats.WaitCount)
	}
}

// collectQueryMetrics 收集查询性能指标
func (pm *PerformanceMonitor) collectQueryMetrics(ctx context.Context) {
	// 查询慢查询统计（需要启用pg_stat_statements）
	var slowQueryCount sql.NullInt64
	err := pm.db.WithContext(ctx).Raw(`
		SELECT COUNT(*) 
		FROM pg_stat_statements 
		WHERE mean_exec_time > ?
	`, pm.alertThresholds.SlowQueryThreshold.Milliseconds()).Scan(&slowQueryCount).Error

	if err == nil && slowQueryCount.Valid {
		pm.metrics.SlowQueryCount = slowQueryCount.Int64
	}

	// 查询平均执行时间
	var avgTime sql.NullFloat64
	err = pm.db.WithContext(ctx).Raw(`
		SELECT AVG(mean_exec_time) 
		FROM pg_stat_statements 
		WHERE calls > 10
	`).Scan(&avgTime).Error

	if err == nil && avgTime.Valid {
		pm.metrics.AvgQueryTime = time.Duration(avgTime.Float64) * time.Millisecond
	}

	// 查询死锁统计
	var deadlocks sql.NullInt64
	err = pm.db.WithContext(ctx).Raw(`
		SELECT deadlocks 
		FROM pg_stat_database 
		WHERE datname = current_database()
	`).Scan(&deadlocks).Error

	if err == nil && deadlocks.Valid {
		pm.metrics.DeadlockCount = deadlocks.Int64
	}
}

// collectBusinessMetrics 收集业务指标
func (pm *PerformanceMonitor) collectBusinessMetrics(ctx context.Context) {
	now := time.Now()
	oneHourAgo := now.Add(-1 * time.Hour)

	// 订单创建率（每小时）
	var orderCount int64
	err := pm.db.WithContext(ctx).Model(&model.TradeOrders{}).
		Where("created_at >= ?", oneHourAgo).
		Count(&orderCount).Error

	if err == nil {
		pm.metrics.OrderCreateRate = float64(orderCount) / 3600 // 转换为每秒
	}

	// 订单成功率
	var totalOrders, successOrders int64
	err = pm.db.WithContext(ctx).Model(&model.TradeOrders{}).
		Where("created_at >= ?", oneHourAgo).
		Count(&totalOrders).Error

	if err == nil && totalOrders > 0 {
		err = pm.db.WithContext(ctx).Model(&model.TradeOrders{}).
			Where("created_at >= ? AND status = ?", oneHourAgo, model.OrderStatusSuccess).
			Count(&successOrders).Error

		if err == nil {
			pm.metrics.OrderSuccessRate = float64(successOrders) / float64(totalOrders) * 100
		}
	}

	// 通知失败率
	var totalNotifies, failedNotifies int64
	err = pm.db.WithContext(ctx).Model(&model.TradeOrders{}).
		Where("created_at >= ? AND status = ? AND notify_num > 0", oneHourAgo, model.OrderStatusSuccess).
		Count(&totalNotifies).Error

	if err == nil && totalNotifies > 0 {
		err = pm.db.WithContext(ctx).Model(&model.TradeOrders{}).
			Where("created_at >= ? AND status = ? AND notify_num > 0 AND notify_state = ?", 
				oneHourAgo, model.OrderStatusSuccess, model.OrderNotifyStateFail).
			Count(&failedNotifies).Error

		if err == nil {
			pm.metrics.NotifyFailureRate = float64(failedNotifies) / float64(totalNotifies) * 100
		}
	}

	// 过期订单数量
	var expiredCount int64
	err = pm.db.WithContext(ctx).Model(&model.TradeOrders{}).
		Where("status = ? AND expired_at < ?", model.OrderStatusWaiting, now).
		Count(&expiredCount).Error

	if err == nil {
		pm.metrics.ExpiredOrderCount = expiredCount
	}
}

// analyzeMetricsAndAlert 分析指标并生成告警
func (pm *PerformanceMonitor) analyzeMetricsAndAlert() {
	// 连接使用率告警
	if pm.metrics.ConnectionUsageRate >= pm.alertThresholds.ConnectionUsageCritical {
		pm.sendAlert(Alert{
			Type:      "CRITICAL",
			Message:   fmt.Sprintf("Database connection usage is critically high: %.2f%%", pm.metrics.ConnectionUsageRate),
			Component: "DATABASE",
			Metric:    "connection_usage_rate",
			Value:     pm.metrics.ConnectionUsageRate,
			Threshold: pm.alertThresholds.ConnectionUsageCritical,
			Timestamp: time.Now(),
		})
	} else if pm.metrics.ConnectionUsageRate >= pm.alertThresholds.ConnectionUsageWarning {
		pm.sendAlert(Alert{
			Type:      "WARNING",
			Message:   fmt.Sprintf("Database connection usage is high: %.2f%%", pm.metrics.ConnectionUsageRate),
			Component: "DATABASE",
			Metric:    "connection_usage_rate",
			Value:     pm.metrics.ConnectionUsageRate,
			Threshold: pm.alertThresholds.ConnectionUsageWarning,
			Timestamp: time.Now(),
		})
	}

	// 等待时间告警
	if pm.metrics.AvgWaitTime >= pm.alertThresholds.WaitTimeCritical {
		pm.sendAlert(Alert{
			Type:      "CRITICAL",
			Message:   fmt.Sprintf("Database connection wait time is critically high: %v", pm.metrics.AvgWaitTime),
			Component: "DATABASE",
			Metric:    "avg_wait_time",
			Value:     pm.metrics.AvgWaitTime,
			Threshold: pm.alertThresholds.WaitTimeCritical,
			Timestamp: time.Now(),
		})
	} else if pm.metrics.AvgWaitTime >= pm.alertThresholds.WaitTimeWarning {
		pm.sendAlert(Alert{
			Type:      "WARNING",
			Message:   fmt.Sprintf("Database connection wait time is high: %v", pm.metrics.AvgWaitTime),
			Component: "DATABASE",
			Metric:    "avg_wait_time",
			Value:     pm.metrics.AvgWaitTime,
			Threshold: pm.alertThresholds.WaitTimeWarning,
			Timestamp: time.Now(),
		})
	}

	// 订单成功率告警
	if pm.metrics.OrderSuccessRate <= pm.alertThresholds.OrderSuccessRateCritical {
		pm.sendAlert(Alert{
			Type:      "CRITICAL",
			Message:   fmt.Sprintf("Order success rate is critically low: %.2f%%", pm.metrics.OrderSuccessRate),
			Component: "BUSINESS",
			Metric:    "order_success_rate",
			Value:     pm.metrics.OrderSuccessRate,
			Threshold: pm.alertThresholds.OrderSuccessRateCritical,
			Timestamp: time.Now(),
		})
	} else if pm.metrics.OrderSuccessRate <= pm.alertThresholds.OrderSuccessRateWarning {
		pm.sendAlert(Alert{
			Type:      "WARNING",
			Message:   fmt.Sprintf("Order success rate is low: %.2f%%", pm.metrics.OrderSuccessRate),
			Component: "BUSINESS",
			Metric:    "order_success_rate",
			Value:     pm.metrics.OrderSuccessRate,
			Threshold: pm.alertThresholds.OrderSuccessRateWarning,
			Timestamp: time.Now(),
		})
	}

	// 通知失败率告警
	if pm.metrics.NotifyFailureRate >= pm.alertThresholds.NotifyFailureRateCritical {
		pm.sendAlert(Alert{
			Type:      "CRITICAL",
			Message:   fmt.Sprintf("Notification failure rate is critically high: %.2f%%", pm.metrics.NotifyFailureRate),
			Component: "BUSINESS",
			Metric:    "notify_failure_rate",
			Value:     pm.metrics.NotifyFailureRate,
			Threshold: pm.alertThresholds.NotifyFailureRateCritical,
			Timestamp: time.Now(),
		})
	} else if pm.metrics.NotifyFailureRate >= pm.alertThresholds.NotifyFailureRateWarning {
		pm.sendAlert(Alert{
			Type:      "WARNING",
			Message:   fmt.Sprintf("Notification failure rate is high: %.2f%%", pm.metrics.NotifyFailureRate),
			Component: "BUSINESS",
			Metric:    "notify_failure_rate",
			Value:     pm.metrics.NotifyFailureRate,
			Threshold: pm.alertThresholds.NotifyFailureRateWarning,
			Timestamp: time.Now(),
		})
	}

	// 过期订单告警
	if pm.metrics.ExpiredOrderCount > 100 {
		pm.sendAlert(Alert{
			Type:      "WARNING",
			Message:   fmt.Sprintf("High number of expired orders need cleanup: %d", pm.metrics.ExpiredOrderCount),
			Component: "BUSINESS",
			Metric:    "expired_order_count",
			Value:     pm.metrics.ExpiredOrderCount,
			Threshold: 100,
			Timestamp: time.Now(),
		})
	}

	// 死锁告警
	if pm.metrics.DeadlockCount > 0 {
		pm.sendAlert(Alert{
			Type:      "WARNING",
			Message:   fmt.Sprintf("Database deadlocks detected: %d", pm.metrics.DeadlockCount),
			Component: "DATABASE",
			Metric:    "deadlock_count",
			Value:     pm.metrics.DeadlockCount,
			Threshold: 0,
			Timestamp: time.Now(),
		})
	}
}

// sendAlert 发送告警
func (pm *PerformanceMonitor) sendAlert(alert Alert) {
	select {
	case pm.alertChannel <- alert:
		// 告警发送成功
	default:
		log.Printf("Alert channel is full, dropping alert: %s", alert.Message)
	}
}

// handleAlerts 处理告警
func (pm *PerformanceMonitor) handleAlerts(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-pm.stopChannel:
			return
		case alert, ok := <-pm.alertChannel:
			if !ok {
				return
			}
			pm.processAlert(alert)
		}
	}
}

// processAlert 处理单个告警
func (pm *PerformanceMonitor) processAlert(alert Alert) {
	// 记录日志
	log.Printf("[%s] %s: %s", alert.Type, alert.Component, alert.Message)

	// TODO: 这里可以集成外部告警系统
	// 例如：发送到Telegram、邮件、Slack、PagerDuty等
	
	// 示例：如果配置了Telegram Bot，发送告警消息
	// if telegramBot := getTelegramBot(); telegramBot != nil {
	//     telegramBot.SendAlert(alert)
	// }

	// 根据告警类型采取自动化响应
	pm.autoRespond(alert)
}

// autoRespond 自动响应告警
func (pm *PerformanceMonitor) autoRespond(alert Alert) {
	switch alert.Metric {
	case "connection_usage_rate":
		if alert.Type == "CRITICAL" {
			// 尝试优化连接池
			if err := model.OptimizeConnectionPool(); err != nil {
				log.Printf("Failed to optimize connection pool: %v", err)
			} else {
				log.Println("Automatically optimized connection pool due to high usage")
			}
		}
	case "expired_order_count":
		if alert.Value.(int64) > 100 {
			// 自动清理过期订单
			dis := NewDataIntegrityService(pm.db)
			if err := dis.ExpireOldOrders(context.Background()); err != nil {
				log.Printf("Failed to auto-expire old orders: %v", err)
			} else {
				log.Println("Automatically cleaned up expired orders")
			}
		}
	}
}

// GetMetrics 获取当前性能指标
func (pm *PerformanceMonitor) GetMetrics() *PerformanceMetrics {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	// 返回指标的副本
	metricsCopy := *pm.metrics
	return &metricsCopy
}

// GetAlertThresholds 获取告警阈值配置
func (pm *PerformanceMonitor) GetAlertThresholds() AlertThresholds {
	return pm.alertThresholds
}

// SetAlertThresholds 设置告警阈值配置
func (pm *PerformanceMonitor) SetAlertThresholds(thresholds AlertThresholds) {
	pm.alertThresholds = thresholds
}

// IsRunning 检查监控是否正在运行
func (pm *PerformanceMonitor) IsRunning() bool {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	return pm.isRunning
}

// GetHealthStatus 获取健康状态
func (pm *PerformanceMonitor) GetHealthStatus() map[string]interface{} {
	metrics := pm.GetMetrics()
	
	status := map[string]interface{}{
		"status": "healthy",
		"checks": make(map[string]interface{}),
	}

	checks := status["checks"].(map[string]interface{})
	overallHealthy := true

	// 检查连接池健康
	if metrics.ConnectionUsageRate >= pm.alertThresholds.ConnectionUsageCritical {
		checks["connection_pool"] = map[string]interface{}{
			"status": "critical",
			"value":  metrics.ConnectionUsageRate,
			"message": "Connection usage critically high",
		}
		overallHealthy = false
	} else if metrics.ConnectionUsageRate >= pm.alertThresholds.ConnectionUsageWarning {
		checks["connection_pool"] = map[string]interface{}{
			"status": "warning",
			"value":  metrics.ConnectionUsageRate,
			"message": "Connection usage high",
		}
	} else {
		checks["connection_pool"] = map[string]interface{}{
			"status":  "healthy",
			"value":   metrics.ConnectionUsageRate,
			"message": "Connection pool operating normally",
		}
	}

	// 检查订单成功率
	if metrics.OrderSuccessRate <= pm.alertThresholds.OrderSuccessRateCritical {
		checks["order_success"] = map[string]interface{}{
			"status":  "critical",
			"value":   metrics.OrderSuccessRate,
			"message": "Order success rate critically low",
		}
		overallHealthy = false
	} else if metrics.OrderSuccessRate <= pm.alertThresholds.OrderSuccessRateWarning {
		checks["order_success"] = map[string]interface{}{
			"status":  "warning",
			"value":   metrics.OrderSuccessRate,
			"message": "Order success rate low",
		}
	} else {
		checks["order_success"] = map[string]interface{}{
			"status":  "healthy",
			"value":   metrics.OrderSuccessRate,
			"message": "Order success rate normal",
		}
	}

	if overallHealthy {
		status["status"] = "healthy"
	} else {
		status["status"] = "unhealthy"
	}

	status["last_updated"] = metrics.LastUpdated
	status["uptime_seconds"] = metrics.UptimeSeconds

	return status
}