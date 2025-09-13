package main

import (
	"USDTMore/app/config"
	"USDTMore/app/model"
	"USDTMore/app/service"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"
)

const (
	helpText = `Database Administration Tool for USDT Payment System

Usage: db_admin [command] [options]

Commands:
  status           - Show database status and connection pool info
  integrity        - Run data integrity check
  repair          - Repair data integrity issues
  optimize        - Optimize database performance
  monitor         - Start performance monitoring
  backup          - Create database backup
  restore         - Restore from backup
  health          - Show health check results
  migrate         - Run database migrations
  analyze         - Analyze query performance
  cleanup         - Clean up old data
  help            - Show this help message

Examples:
  db_admin status
  db_admin integrity --fix
  db_admin backup --type=full
  db_admin restore --file=/path/to/backup.sql
  db_admin monitor --duration=60s
  db_admin cleanup --days=30

Environment Variables:
  DB_HOST, DB_PORT, DB_NAME, DB_USER, DB_PASSWORD
  DB_MONITORING_ENABLED, DB_BACKUP_DIR
`
)

var (
	// Global flags
	fixFlag       = flag.Bool("fix", false, "Automatically fix detected issues")
	typeFlag      = flag.String("type", "full", "Type for backup/restore operations")
	fileFlag      = flag.String("file", "", "File path for backup/restore operations")
	durationFlag  = flag.Duration("duration", 60*time.Second, "Duration for monitoring")
	daysFlag      = flag.Int("days", 7, "Number of days for cleanup operations")
	formatFlag    = flag.String("format", "table", "Output format: table, json")
	verboseFlag   = flag.Bool("verbose", false, "Verbose output")
)

func main() {
	flag.Parse()
	
	if len(os.Args) < 2 {
		fmt.Print(helpText)
		os.Exit(1)
	}

	command := os.Args[1]
	
	// Initialize database connection
	if err := model.Init(); err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer model.Close()

	switch command {
	case "status":
		showStatus()
	case "integrity":
		runIntegrityCheck()
	case "repair":
		repairIntegrityIssues()
	case "optimize":
		optimizeDatabase()
	case "monitor":
		startMonitoring()
	case "backup":
		createBackup()
	case "restore":
		restoreDatabase()
	case "health":
		showHealthCheck()
	case "migrate":
		runMigrations()
	case "analyze":
		analyzePerformance()
	case "cleanup":
		cleanupOldData()
	case "help":
		fmt.Print(helpText)
	default:
		fmt.Printf("Unknown command: %s\n\n", command)
		fmt.Print(helpText)
		os.Exit(1)
	}
}

func showStatus() {
	fmt.Println("=== Database Status ===")
	
	// Test connection
	if err := model.Ping(); err != nil {
		fmt.Printf("❌ Database connection: FAILED (%v)\n", err)
		return
	}
	fmt.Println("✅ Database connection: OK")

	// Connection pool stats
	stats, err := model.GetConnectionPoolStats()
	if err != nil {
		fmt.Printf("❌ Failed to get connection stats: %v\n", err)
		return
	}

	fmt.Printf("\n=== Connection Pool ===\n")
	fmt.Printf("Open Connections: %d\n", stats.OpenConnections)
	fmt.Printf("In Use: %d\n", stats.InUse)
	fmt.Printf("Idle: %d\n", stats.Idle)
	fmt.Printf("Max Open: %d\n", stats.MaxOpenConnections)
	fmt.Printf("Max Idle: %d\n", stats.MaxIdleConns)
	fmt.Printf("Wait Count: %d\n", stats.WaitCount)
	
	if stats.MaxOpenConnections > 0 {
		usageRate := float64(stats.InUse) / float64(stats.MaxOpenConnections) * 100
		fmt.Printf("Usage Rate: %.2f%%\n", usageRate)
	}

	// Database configuration
	fmt.Printf("\n=== Configuration ===\n")
	fmt.Printf("Host: %s\n", config.GetPostgreSQLHost())
	fmt.Printf("Port: %s\n", config.GetPostgreSQLPort())
	fmt.Printf("Database: %s\n", config.GetPostgreSQLDatabase())
	fmt.Printf("Max Idle Conns: %d\n", config.GetDbMaxIdleConns())
	fmt.Printf("Max Open Conns: %d\n", config.GetDbMaxOpenConns())
	fmt.Printf("Conn Max Lifetime: %v\n", config.GetDbConnMaxLifetime())
	fmt.Printf("Monitoring Enabled: %t\n", config.IsDbMonitoringEnabled())
}

func runIntegrityCheck() {
	fmt.Println("=== Running Data Integrity Check ===")
	
	dis := service.NewDataIntegrityService(model.DB)
	ctx := context.Background()
	
	results, err := dis.RunFullIntegrityCheck(ctx)
	if err != nil {
		fmt.Printf("❌ Integrity check failed: %v\n", err)
		os.Exit(1)
	}

	// Count results by status
	passCount := 0
	warningCount := 0
	failCount := 0

	for _, result := range results {
		switch result.Status {
		case "PASS":
			passCount++
			if *verboseFlag {
				fmt.Printf("✅ %s: PASS\n", result.CheckName)
			}
		case "WARNING":
			warningCount++
			fmt.Printf("⚠️  %s: WARNING\n", result.CheckName)
			for _, issue := range result.Issues {
				fmt.Printf("   - %s\n", issue)
			}
			if result.FixSuggestion != "" {
				fmt.Printf("   💡 %s\n", result.FixSuggestion)
			}
		case "FAIL":
			failCount++
			fmt.Printf("❌ %s: FAIL\n", result.CheckName)
			for _, issue := range result.Issues {
				fmt.Printf("   - %s\n", issue)
			}
			if result.FixSuggestion != "" {
				fmt.Printf("   💡 %s\n", result.FixSuggestion)
			}
		}
		fmt.Println()
	}

	fmt.Printf("=== Summary ===\n")
	fmt.Printf("Total Checks: %d\n", len(results))
	fmt.Printf("✅ Passed: %d\n", passCount)
	fmt.Printf("⚠️  Warnings: %d\n", warningCount)
	fmt.Printf("❌ Failed: %d\n", failCount)

	// Auto-fix if requested
	if *fixFlag && (warningCount > 0 || failCount > 0) {
		fmt.Println("\n=== Auto-fixing Issues ===")
		repairIntegrityIssues()
	}

	// Exit with error code if there are failures
	if failCount > 0 {
		os.Exit(1)
	}
}

func repairIntegrityIssues() {
	fmt.Println("=== Repairing Data Integrity Issues ===")
	
	dis := service.NewDataIntegrityService(model.DB)
	ctx := context.Background()

	// Fix common issues
	repairs := []struct {
		name string
		fn   func(context.Context) error
	}{
		{"Expire old orders", dis.ExpireOldOrders},
		{"Fix negative version numbers", dis.FixNegativeVersionNumbers},
		{"Cleanup orphaned notify records", dis.CleanupOrphanedNotifyRecords},
		{"Repair order ID duplicates", dis.RepairOrderIdDuplicates},
	}

	for _, repair := range repairs {
		fmt.Printf("Running: %s...", repair.name)
		if err := repair.fn(ctx); err != nil {
			fmt.Printf(" ❌ FAILED: %v\n", err)
		} else {
			fmt.Printf(" ✅ SUCCESS\n")
		}
	}

	fmt.Println("✅ Repair operations completed")
}

func optimizeDatabase() {
	fmt.Println("=== Optimizing Database Performance ===")

	// Optimize connection pool
	fmt.Print("Optimizing connection pool...")
	if err := model.OptimizeConnectionPool(); err != nil {
		fmt.Printf(" ❌ FAILED: %v\n", err)
	} else {
		fmt.Printf(" ✅ SUCCESS\n")
	}

	// Run ANALYZE
	fmt.Print("Updating table statistics...")
	if err := model.DB.Exec("ANALYZE").Error; err != nil {
		fmt.Printf(" ❌ FAILED: %v\n", err)
	} else {
		fmt.Printf(" ✅ SUCCESS\n")
	}

	// Check for missing indexes (if needed)
	fmt.Print("Checking index usage...")
	// This would require custom logic to analyze query patterns
	fmt.Printf(" ℹ️  Check logs for slow queries\n")

	fmt.Println("✅ Optimization completed")
}

func startMonitoring() {
	fmt.Printf("=== Starting Performance Monitoring (%v) ===\n", *durationFlag)
	
	monitor := service.NewPerformanceMonitor(model.DB)
	ctx, cancel := context.WithTimeout(context.Background(), *durationFlag)
	defer cancel()

	if err := monitor.Start(ctx); err != nil {
		fmt.Printf("❌ Failed to start monitoring: %v\n", err)
		os.Exit(1)
	}

	// Display metrics periodically
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			fmt.Println("\n⏰ Monitoring completed")
			return
		case <-ticker.C:
			metrics := monitor.GetMetrics()
			fmt.Printf("\n[%s] Performance Metrics:\n", time.Now().Format("15:04:05"))
			fmt.Printf("  Connection Usage: %.2f%% (%d/%d)\n", 
				metrics.ConnectionUsageRate, metrics.InUseConnections, metrics.OpenConnections)
			fmt.Printf("  Avg Wait Time: %v\n", metrics.AvgWaitTime)
			fmt.Printf("  Order Success Rate: %.2f%%\n", metrics.OrderSuccessRate)
			fmt.Printf("  Notify Failure Rate: %.2f%%\n", metrics.NotifyFailureRate)
			fmt.Printf("  Slow Queries: %d\n", metrics.SlowQueryCount)
		}
	}
}

func createBackup() {
	fmt.Printf("=== Creating Database Backup (Type: %s) ===\n", *typeFlag)
	
	script := "./scripts/backup_database.sh"
	if _, err := os.Stat(script); os.IsNotExist(err) {
		fmt.Printf("❌ Backup script not found: %s\n", script)
		os.Exit(1)
	}

	fmt.Printf("Executing: %s %s\n", script, *typeFlag)
	// In real implementation, would execute the script
	fmt.Printf("ℹ️  Please run: %s %s\n", script, *typeFlag)
}

func restoreDatabase() {
	if *fileFlag == "" {
		fmt.Println("❌ Restore file not specified. Use --file flag")
		os.Exit(1)
	}

	fmt.Printf("=== Restoring Database (Type: %s, File: %s) ===\n", *typeFlag, *fileFlag)
	
	script := "./scripts/restore_database.sh"
	if _, err := os.Stat(script); os.IsNotExist(err) {
		fmt.Printf("❌ Restore script not found: %s\n", script)
		os.Exit(1)
	}

	fmt.Printf("⚠️  This operation will modify the database!\n")
	fmt.Printf("Executing: %s %s %s\n", script, *typeFlag, *fileFlag)
	// In real implementation, would execute the script
	fmt.Printf("ℹ️  Please run: %s %s %s\n", script, *typeFlag, *fileFlag)
}

func showHealthCheck() {
	fmt.Println("=== Database Health Check ===")

	monitor := service.NewPerformanceMonitor(model.DB)
	health := monitor.GetHealthStatus()

	if *formatFlag == "json" {
		data, _ := json.MarshalIndent(health, "", "  ")
		fmt.Println(string(data))
		return
	}

	// Table format
	status := health["status"].(string)
	if status == "healthy" {
		fmt.Println("✅ Overall Status: HEALTHY")
	} else {
		fmt.Println("❌ Overall Status: UNHEALTHY")
	}

	fmt.Printf("Last Updated: %v\n", health["last_updated"])
	fmt.Printf("Uptime: %d seconds\n", health["uptime_seconds"])

	checks := health["checks"].(map[string]interface{})
	fmt.Printf("\n=== Component Health ===\n")
	
	for component, check := range checks {
		checkData := check.(map[string]interface{})
		status := checkData["status"].(string)
		message := checkData["message"].(string)
		
		var icon string
		switch status {
		case "healthy":
			icon = "✅"
		case "warning":
			icon = "⚠️"
		case "critical":
			icon = "❌"
		default:
			icon = "❓"
		}
		
		fmt.Printf("%s %s: %s - %s\n", icon, 
			strings.ToUpper(component), 
			strings.ToUpper(status), 
			message)
	}
}

func runMigrations() {
	fmt.Println("=== Running Database Migrations ===")
	
	if model.IsMigrationInProgress() {
		fmt.Println("⚠️  Migration is already in progress")
		return
	}

	ctx := context.Background()
	if err := model.AutoMigrateWithContext(ctx); err != nil {
		fmt.Printf("❌ Migration failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("✅ Database migrations completed successfully")
}

func analyzePerformance() {
	fmt.Println("=== Analyzing Query Performance ===")
	
	// This would require pg_stat_statements extension
	queries := []string{
		`SELECT 
			query,
			calls,
			total_exec_time,
			mean_exec_time,
			rows
		FROM pg_stat_statements 
		WHERE mean_exec_time > 100
		ORDER BY mean_exec_time DESC 
		LIMIT 10`,
		
		`SELECT 
			schemaname,
			tablename,
			seq_scan,
			seq_tup_read,
			idx_scan,
			idx_tup_fetch
		FROM pg_stat_user_tables`,
	}

	for i, query := range queries {
		fmt.Printf("\n=== Query %d ===\n", i+1)
		
		rows, err := model.DB.Raw(query).Rows()
		if err != nil {
			fmt.Printf("❌ Query failed: %v\n", err)
			continue
		}
		defer rows.Close()

		columns, _ := rows.Columns()
		fmt.Printf("Columns: %v\n", columns)
		
		rowCount := 0
		for rows.Next() {
			rowCount++
			if rowCount <= 5 { // Show first 5 rows
				values := make([]interface{}, len(columns))
				valuePtrs := make([]interface{}, len(columns))
				for i := range values {
					valuePtrs[i] = &values[i]
				}
				
				if err := rows.Scan(valuePtrs...); err != nil {
					continue
				}
				
				fmt.Printf("Row %d: %v\n", rowCount, values)
			}
		}
		
		if rowCount == 0 {
			fmt.Println("ℹ️  No results (pg_stat_statements extension may not be enabled)")
		} else if rowCount > 5 {
			fmt.Printf("... and %d more rows\n", rowCount-5)
		}
	}
}

func cleanupOldData() {
	fmt.Printf("=== Cleaning Up Data Older Than %d Days ===\n", *daysFlag)
	
	cutoffDate := time.Now().AddDate(0, 0, -*daysFlag)
	
	// Clean up old notify records
	fmt.Print("Cleaning notify records...")
	result := model.DB.Where("created_at < ?", cutoffDate).Delete(&model.NotifyRecord{})
	if result.Error != nil {
		fmt.Printf(" ❌ FAILED: %v\n", result.Error)
	} else {
		fmt.Printf(" ✅ SUCCESS (%d records removed)\n", result.RowsAffected)
	}

	// Clean up expired orders older than cutoff
	fmt.Print("Cleaning expired orders...")
	result = model.DB.Where("status = ? AND created_at < ?", model.OrderStatusExpired, cutoffDate).Delete(&model.TradeOrders{})
	if result.Error != nil {
		fmt.Printf(" ❌ FAILED: %v\n", result.Error)
	} else {
		fmt.Printf(" ✅ SUCCESS (%d records removed)\n", result.RowsAffected)
	}

	// VACUUM to reclaim space
	fmt.Print("Reclaiming disk space...")
	if err := model.DB.Exec("VACUUM ANALYZE").Error; err != nil {
		fmt.Printf(" ❌ FAILED: %v\n", err)
	} else {
		fmt.Printf(" ✅ SUCCESS\n")
	}

	fmt.Println("✅ Cleanup completed")
}