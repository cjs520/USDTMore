package service

import (
	"USDTMore/app/model"
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"gorm.io/gorm"
)

// DataIntegrityService 数据完整性服务
type DataIntegrityService struct {
	db *gorm.DB
}

// NewDataIntegrityService 创建数据完整性服务
func NewDataIntegrityService(db *gorm.DB) *DataIntegrityService {
	return &DataIntegrityService{db: db}
}

// IntegrityCheckResult 完整性检查结果
type IntegrityCheckResult struct {
	CheckName     string                 `json:"check_name"`
	Status        string                 `json:"status"` // PASS, FAIL, WARNING
	Issues        []string               `json:"issues"`
	FixSuggestion string                 `json:"fix_suggestion"`
	Details       map[string]interface{} `json:"details"`
	CheckedAt     time.Time              `json:"checked_at"`
}

// RunFullIntegrityCheck 执行完整的数据完整性检查
func (dis *DataIntegrityService) RunFullIntegrityCheck(ctx context.Context) ([]IntegrityCheckResult, error) {
	var results []IntegrityCheckResult

	// 检查列表
	checks := []func(context.Context) IntegrityCheckResult{
		dis.CheckOrderIdUniqueness,
		dis.CheckTradeIdUniqueness,
		dis.CheckTradeHashIntegrity,
		dis.CheckAmountConsistency,
		dis.CheckStatusConsistency,
		dis.CheckVersionIntegrity,
		dis.CheckExpiredOrdersCleanup,
		dis.CheckWalletAddressConsistency,
		dis.CheckNotifyRecordConsistency,
		dis.CheckOrphanedRecords,
	}

	for _, check := range checks {
		result := check(ctx)
		results = append(results, result)
		
		// 记录检查结果
		if result.Status == "FAIL" {
			log.Printf("Data integrity check FAILED: %s - %v", result.CheckName, result.Issues)
		} else if result.Status == "WARNING" {
			log.Printf("Data integrity check WARNING: %s - %v", result.CheckName, result.Issues)
		}
	}

	return results, nil
}

// CheckOrderIdUniqueness 检查订单ID唯一性
func (dis *DataIntegrityService) CheckOrderIdUniqueness(ctx context.Context) IntegrityCheckResult {
	result := IntegrityCheckResult{
		CheckName: "OrderIdUniqueness",
		CheckedAt: time.Now(),
		Details:   make(map[string]interface{}),
	}

	// 查找重复的订单ID
	var duplicates []struct {
		OrderId string
		Count   int64
	}

	err := dis.db.WithContext(ctx).
		Model(&model.TradeOrders{}).
		Select("order_id, COUNT(*) as count").
		Group("order_id").
		Having("COUNT(*) > 1").
		Find(&duplicates).Error

	if err != nil {
		result.Status = "FAIL"
		result.Issues = []string{fmt.Sprintf("Query error: %v", err)}
		return result
	}

	if len(duplicates) > 0 {
		result.Status = "FAIL"
		for _, dup := range duplicates {
			result.Issues = append(result.Issues, 
				fmt.Sprintf("OrderId '%s' appears %d times", dup.OrderId, dup.Count))
		}
		result.FixSuggestion = "Run RepairOrderIdDuplicates to fix duplicate order IDs"
		result.Details["duplicate_count"] = len(duplicates)
	} else {
		result.Status = "PASS"
	}

	return result
}

// CheckTradeIdUniqueness 检查交易ID唯一性
func (dis *DataIntegrityService) CheckTradeIdUniqueness(ctx context.Context) IntegrityCheckResult {
	result := IntegrityCheckResult{
		CheckName: "TradeIdUniqueness",
		CheckedAt: time.Now(),
		Details:   make(map[string]interface{}),
	}

	var duplicates []struct {
		TradeId string
		Count   int64
	}

	err := dis.db.WithContext(ctx).
		Model(&model.TradeOrders{}).
		Select("trade_id, COUNT(*) as count").
		Group("trade_id").
		Having("COUNT(*) > 1").
		Find(&duplicates).Error

	if err != nil {
		result.Status = "FAIL"
		result.Issues = []string{fmt.Sprintf("Query error: %v", err)}
		return result
	}

	if len(duplicates) > 0 {
		result.Status = "FAIL"
		for _, dup := range duplicates {
			result.Issues = append(result.Issues,
				fmt.Sprintf("TradeId '%s' appears %d times", dup.TradeId, dup.Count))
		}
		result.FixSuggestion = "Run RepairTradeIdDuplicates to fix duplicate trade IDs"
		result.Details["duplicate_count"] = len(duplicates)
	} else {
		result.Status = "PASS"
	}

	return result
}

// CheckTradeHashIntegrity 检查交易哈希完整性
func (dis *DataIntegrityService) CheckTradeHashIntegrity(ctx context.Context) IntegrityCheckResult {
	result := IntegrityCheckResult{
		CheckName: "TradeHashIntegrity",
		CheckedAt: time.Now(),
		Details:   make(map[string]interface{}),
	}

	// 检查成功订单没有交易哈希
	var missingHashCount int64
	err := dis.db.WithContext(ctx).
		Model(&model.TradeOrders{}).
		Where("status = ? AND (trade_hash = '' OR trade_hash IS NULL)", model.OrderStatusSuccess).
		Count(&missingHashCount).Error

	if err != nil {
		result.Status = "FAIL"
		result.Issues = []string{fmt.Sprintf("Query error: %v", err)}
		return result
	}

	// 检查重复的交易哈希
	var hashDuplicates []struct {
		TradeHash string
		Count     int64
	}

	err = dis.db.WithContext(ctx).
		Model(&model.TradeOrders{}).
		Select("trade_hash, COUNT(*) as count").
		Where("trade_hash != '' AND trade_hash IS NOT NULL").
		Group("trade_hash").
		Having("COUNT(*) > 1").
		Find(&hashDuplicates).Error

	if err != nil {
		result.Status = "FAIL"
		result.Issues = append(result.Issues, fmt.Sprintf("Hash duplicate query error: %v", err))
		return result
	}

	if missingHashCount > 0 || len(hashDuplicates) > 0 {
		result.Status = "FAIL"
		
		if missingHashCount > 0 {
			result.Issues = append(result.Issues,
				fmt.Sprintf("%d successful orders missing trade hash", missingHashCount))
		}
		
		for _, dup := range hashDuplicates {
			result.Issues = append(result.Issues,
				fmt.Sprintf("TradeHash '%s' appears %d times", dup.TradeHash, dup.Count))
		}
		
		result.FixSuggestion = "Investigate missing trade hashes and duplicate hashes"
		result.Details["missing_hash_count"] = missingHashCount
		result.Details["duplicate_hash_count"] = len(hashDuplicates)
	} else {
		result.Status = "PASS"
	}

	return result
}

// CheckAmountConsistency 检查金额一致性
func (dis *DataIntegrityService) CheckAmountConsistency(ctx context.Context) IntegrityCheckResult {
	result := IntegrityCheckResult{
		CheckName: "AmountConsistency",
		CheckedAt: time.Now(),
		Details:   make(map[string]interface{}),
	}

	// 检查金额为0或负数的订单
	var invalidAmountCount int64
	err := dis.db.WithContext(ctx).
		Model(&model.TradeOrders{}).
		Where("money <= 0").
		Count(&invalidAmountCount).Error

	if err != nil {
		result.Status = "FAIL"
		result.Issues = []string{fmt.Sprintf("Query error: %v", err)}
		return result
	}

	// 检查USDT金额格式
	var invalidUsdtCount int64
	err = dis.db.WithContext(ctx).
		Model(&model.TradeOrders{}).
		Where("amount = '' OR amount IS NULL OR amount = '0' OR amount = '0.00'").
		Count(&invalidUsdtCount).Error

	if err != nil {
		result.Status = "FAIL"
		result.Issues = append(result.Issues, fmt.Sprintf("USDT amount query error: %v", err))
		return result
	}

	if invalidAmountCount > 0 || invalidUsdtCount > 0 {
		result.Status = "FAIL"
		
		if invalidAmountCount > 0 {
			result.Issues = append(result.Issues,
				fmt.Sprintf("%d orders with invalid money amount", invalidAmountCount))
		}
		
		if invalidUsdtCount > 0 {
			result.Issues = append(result.Issues,
				fmt.Sprintf("%d orders with invalid USDT amount", invalidUsdtCount))
		}
		
		result.FixSuggestion = "Review and fix invalid amount data"
		result.Details["invalid_money_count"] = invalidAmountCount
		result.Details["invalid_usdt_count"] = invalidUsdtCount
	} else {
		result.Status = "PASS"
	}

	return result
}

// CheckStatusConsistency 检查状态一致性
func (dis *DataIntegrityService) CheckStatusConsistency(ctx context.Context) IntegrityCheckResult {
	result := IntegrityCheckResult{
		CheckName: "StatusConsistency",
		CheckedAt: time.Now(),
		Details:   make(map[string]interface{}),
	}

	// 检查无效状态
	var invalidStatusCount int64
	err := dis.db.WithContext(ctx).
		Model(&model.TradeOrders{}).
		Where("status NOT IN (?, ?, ?)", 
			model.OrderStatusWaiting, model.OrderStatusSuccess, model.OrderStatusExpired).
		Count(&invalidStatusCount).Error

	if err != nil {
		result.Status = "FAIL"
		result.Issues = []string{fmt.Sprintf("Query error: %v", err)}
		return result
	}

	// 检查状态与时间的一致性
	var expiredButNotMarkedCount int64
	err = dis.db.WithContext(ctx).
		Model(&model.TradeOrders{}).
		Where("status = ? AND expired_at < ?", model.OrderStatusWaiting, time.Now()).
		Count(&expiredButNotMarkedCount).Error

	if err != nil {
		result.Status = "FAIL"
		result.Issues = append(result.Issues, fmt.Sprintf("Expired status query error: %v", err))
		return result
	}

	if invalidStatusCount > 0 {
		result.Status = "FAIL"
		result.Issues = append(result.Issues,
			fmt.Sprintf("%d orders with invalid status", invalidStatusCount))
		result.FixSuggestion = "Fix invalid status values"
		result.Details["invalid_status_count"] = invalidStatusCount
	} else if expiredButNotMarkedCount > 0 {
		result.Status = "WARNING"
		result.Issues = append(result.Issues,
			fmt.Sprintf("%d orders expired but not marked as expired", expiredButNotMarkedCount))
		result.FixSuggestion = "Run ExpireOldOrders to clean up expired orders"
		result.Details["expired_but_not_marked_count"] = expiredButNotMarkedCount
	} else {
		result.Status = "PASS"
	}

	return result
}

// CheckVersionIntegrity 检查版本号完整性
func (dis *DataIntegrityService) CheckVersionIntegrity(ctx context.Context) IntegrityCheckResult {
	result := IntegrityCheckResult{
		CheckName: "VersionIntegrity",
		CheckedAt: time.Now(),
		Details:   make(map[string]interface{}),
	}

	// 检查负数版本号
	var negativeVersionCount int64
	err := dis.db.WithContext(ctx).
		Model(&model.TradeOrders{}).
		Where("version < 0").
		Count(&negativeVersionCount).Error

	if err != nil {
		result.Status = "FAIL"
		result.Issues = []string{fmt.Sprintf("Query error: %v", err)}
		return result
	}

	if negativeVersionCount > 0 {
		result.Status = "FAIL"
		result.Issues = append(result.Issues,
			fmt.Sprintf("%d orders with negative version numbers", negativeVersionCount))
		result.FixSuggestion = "Reset negative version numbers to 0"
		result.Details["negative_version_count"] = negativeVersionCount
	} else {
		result.Status = "PASS"
	}

	return result
}

// CheckExpiredOrdersCleanup 检查过期订单清理
func (dis *DataIntegrityService) CheckExpiredOrdersCleanup(ctx context.Context) IntegrityCheckResult {
	result := IntegrityCheckResult{
		CheckName: "ExpiredOrdersCleanup",
		CheckedAt: time.Now(),
		Details:   make(map[string]interface{}),
	}

	// 检查超过24小时的等待中订单
	cutoffTime := time.Now().Add(-24 * time.Hour)
	var oldWaitingCount int64
	err := dis.db.WithContext(ctx).
		Model(&model.TradeOrders{}).
		Where("status = ? AND created_at < ?", model.OrderStatusWaiting, cutoffTime).
		Count(&oldWaitingCount).Error

	if err != nil {
		result.Status = "FAIL"
		result.Issues = []string{fmt.Sprintf("Query error: %v", err)}
		return result
	}

	if oldWaitingCount > 0 {
		result.Status = "WARNING"
		result.Issues = append(result.Issues,
			fmt.Sprintf("%d waiting orders older than 24 hours need cleanup", oldWaitingCount))
		result.FixSuggestion = "Run ExpireOldOrders to clean up old waiting orders"
		result.Details["old_waiting_count"] = oldWaitingCount
	} else {
		result.Status = "PASS"
	}

	return result
}

// CheckWalletAddressConsistency 检查钱包地址一致性
func (dis *DataIntegrityService) CheckWalletAddressConsistency(ctx context.Context) IntegrityCheckResult {
	result := IntegrityCheckResult{
		CheckName: "WalletAddressConsistency",
		CheckedAt: time.Now(),
		Details:   make(map[string]interface{}),
	}

	// 检查重复的链+地址组合
	var duplicates []struct {
		Chain   string
		Address string
		Count   int64
	}

	err := dis.db.WithContext(ctx).
		Model(&model.WalletAddress{}).
		Select("chain, address, COUNT(*) as count").
		Group("chain, address").
		Having("COUNT(*) > 1").
		Find(&duplicates).Error

	if err != nil {
		result.Status = "FAIL"
		result.Issues = []string{fmt.Sprintf("Query error: %v", err)}
		return result
	}

	if len(duplicates) > 0 {
		result.Status = "FAIL"
		for _, dup := range duplicates {
			result.Issues = append(result.Issues,
				fmt.Sprintf("Chain '%s' Address '%s' appears %d times", dup.Chain, dup.Address, dup.Count))
		}
		result.FixSuggestion = "Remove duplicate wallet address entries"
		result.Details["duplicate_count"] = len(duplicates)
	} else {
		result.Status = "PASS"
	}

	return result
}

// CheckNotifyRecordConsistency 检查通知记录一致性
func (dis *DataIntegrityService) CheckNotifyRecordConsistency(ctx context.Context) IntegrityCheckResult {
	result := IntegrityCheckResult{
		CheckName: "NotifyRecordConsistency",
		CheckedAt: time.Now(),
		Details:   make(map[string]interface{}),
	}

	// 检查孤立的通知记录（没有对应的交易订单）
	var orphanedCount int64
	err := dis.db.WithContext(ctx).Raw(`
		SELECT COUNT(*) 
		FROM notify_record nr 
		LEFT JOIN trade_orders to ON nr.txid = to.trade_hash 
		WHERE to.trade_hash IS NULL
	`).Scan(&orphanedCount).Error

	if err != nil {
		result.Status = "FAIL"
		result.Issues = []string{fmt.Sprintf("Query error: %v", err)}
		return result
	}

	if orphanedCount > 0 {
		result.Status = "WARNING"
		result.Issues = append(result.Issues,
			fmt.Sprintf("%d orphaned notify records found", orphanedCount))
		result.FixSuggestion = "Clean up orphaned notify records"
		result.Details["orphaned_count"] = orphanedCount
	} else {
		result.Status = "PASS"
	}

	return result
}

// CheckOrphanedRecords 检查孤立记录
func (dis *DataIntegrityService) CheckOrphanedRecords(ctx context.Context) IntegrityCheckResult {
	result := IntegrityCheckResult{
		CheckName: "OrphanedRecords",
		CheckedAt: time.Now(),
		Details:   make(map[string]interface{}),
	}

	// 检查引用不存在钱包地址的订单
	var orphanedOrdersCount int64
	err := dis.db.WithContext(ctx).Raw(`
		SELECT COUNT(*) 
		FROM trade_orders to 
		LEFT JOIN wallet_address wa ON to.chain = wa.chain AND to.address = wa.address 
		WHERE wa.address IS NULL AND to.status = ?
	`, model.OrderStatusWaiting).Scan(&orphanedOrdersCount).Error

	if err != nil {
		result.Status = "FAIL"
		result.Issues = []string{fmt.Sprintf("Query error: %v", err)}
		return result
	}

	if orphanedOrdersCount > 0 {
		result.Status = "WARNING"
		result.Issues = append(result.Issues,
			fmt.Sprintf("%d orders reference non-existent wallet addresses", orphanedOrdersCount))
		result.FixSuggestion = "Review wallet address configuration"
		result.Details["orphaned_orders_count"] = orphanedOrdersCount
	} else {
		result.Status = "PASS"
	}

	return result
}

// RepairOrderIdDuplicates 修复重复的订单ID
func (dis *DataIntegrityService) RepairOrderIdDuplicates(ctx context.Context) error {
	// 找出重复的订单ID
	var duplicates []struct {
		OrderId string
		Count   int64
	}

	err := dis.db.WithContext(ctx).
		Model(&model.TradeOrders{}).
		Select("order_id, COUNT(*) as count").
		Group("order_id").
		Having("COUNT(*) > 1").
		Find(&duplicates).Error

	if err != nil {
		return fmt.Errorf("failed to find duplicate order IDs: %w", err)
	}

	for _, dup := range duplicates {
		// 获取重复的订单记录
		var orders []model.TradeOrders
		err = dis.db.WithContext(ctx).
			Where("order_id = ?", dup.OrderId).
			Order("created_at ASC").
			Find(&orders).Error

		if err != nil {
			log.Printf("Failed to get duplicate orders for OrderId %s: %v", dup.OrderId, err)
			continue
		}

		// 保留最早的订单，其他订单添加后缀
		for i := 1; i < len(orders); i++ {
			newOrderId := fmt.Sprintf("%s_dup_%d_%d", orders[i].OrderId, i, time.Now().Unix())
			
			err = dis.db.WithContext(ctx).
				Model(&orders[i]).
				Update("order_id", newOrderId).Error

			if err != nil {
				log.Printf("Failed to update duplicate OrderId %s to %s: %v", 
					orders[i].OrderId, newOrderId, err)
			} else {
				log.Printf("Updated duplicate OrderId %s to %s", orders[i].OrderId, newOrderId)
			}
		}
	}

	return nil
}

// ExpireOldOrders 标记过期的订单
func (dis *DataIntegrityService) ExpireOldOrders(ctx context.Context) error {
	result := dis.db.WithContext(ctx).
		Model(&model.TradeOrders{}).
		Where("status = ? AND expired_at < ?", model.OrderStatusWaiting, time.Now()).
		Update("status", model.OrderStatusExpired)

	if result.Error != nil {
		return fmt.Errorf("failed to expire old orders: %w", result.Error)
	}

	log.Printf("Expired %d old orders", result.RowsAffected)
	return nil
}

// CleanupOrphanedNotifyRecords 清理孤立的通知记录
func (dis *DataIntegrityService) CleanupOrphanedNotifyRecords(ctx context.Context) error {
	result := dis.db.WithContext(ctx).Exec(`
		DELETE FROM notify_record 
		WHERE txid NOT IN (
			SELECT DISTINCT trade_hash 
			FROM trade_orders 
			WHERE trade_hash IS NOT NULL AND trade_hash != ''
		)
	`)

	if result.Error != nil {
		return fmt.Errorf("failed to cleanup orphaned notify records: %w", result.Error)
	}

	log.Printf("Cleaned up %d orphaned notify records", result.RowsAffected)
	return nil
}

// FixNegativeVersionNumbers 修复负数版本号
func (dis *DataIntegrityService) FixNegativeVersionNumbers(ctx context.Context) error {
	result := dis.db.WithContext(ctx).
		Model(&model.TradeOrders{}).
		Where("version < 0").
		Update("version", 0)

	if result.Error != nil {
		return fmt.Errorf("failed to fix negative version numbers: %w", result.Error)
	}

	log.Printf("Fixed %d negative version numbers", result.RowsAffected)
	return nil
}

// GenerateIntegrityReport 生成完整性检查报告
func (dis *DataIntegrityService) GenerateIntegrityReport(ctx context.Context) (string, error) {
	results, err := dis.RunFullIntegrityCheck(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to run integrity check: %w", err)
	}

	var report strings.Builder
	report.WriteString("=== Database Integrity Check Report ===\n")
	report.WriteString(fmt.Sprintf("Generated at: %s\n\n", time.Now().Format(time.RFC3339)))

	passCount := 0
	warningCount := 0
	failCount := 0

	for _, result := range results {
		switch result.Status {
		case "PASS":
			passCount++
		case "WARNING":
			warningCount++
		case "FAIL":
			failCount++
		}

		report.WriteString(fmt.Sprintf("Check: %s - %s\n", result.CheckName, result.Status))
		
		if len(result.Issues) > 0 {
			for _, issue := range result.Issues {
				report.WriteString(fmt.Sprintf("  Issue: %s\n", issue))
			}
		}
		
		if result.FixSuggestion != "" {
			report.WriteString(fmt.Sprintf("  Suggestion: %s\n", result.FixSuggestion))
		}
		
		report.WriteString("\n")
	}

	report.WriteString("=== Summary ===\n")
	report.WriteString(fmt.Sprintf("Total Checks: %d\n", len(results)))
	report.WriteString(fmt.Sprintf("Passed: %d\n", passCount))
	report.WriteString(fmt.Sprintf("Warnings: %d\n", warningCount))
	report.WriteString(fmt.Sprintf("Failed: %d\n", failCount))

	return report.String(), nil
}