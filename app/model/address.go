package model

import (
	"USDTMore/app/config"
	"USDTMore/app/help"
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
)

const StatusEnable = 1
const StatusDisable = 0
const OtherNotifyEnable = 1
const OtherNotifyDisable = 0

type WalletAddress struct {
	Id          int64     `gorm:"integer;primaryKey;not null;comment:id"`
	Chain       string    `gorm:"type:varchar(255);not null;comment:链路名称 TRON POLY OP BSC"`
	StartBlock  int64     `gorm:"integer;not null;default:0;comment:初始化块，每次查询记录一天之前的blocknum"`
	InAmount    float64   `gorm:"type:decimal(20,8);not null;default:0;comment:累计转入"`
	OutAmount   float64   `gorm:"type:decimal(20,8);not null;default:0;comment:累计转出"`
	Count       int64     `gorm:"integer;not null;default:0;comment:历史订单数量"`
	Address     string    `gorm:"type:varchar(64);not null;comment:钱包地址"`
	Status      int       `gorm:"type:smallint;not null;default:1;comment:地址状态 1启动 0禁止"`
	OtherNotify int       `gorm:"type:smallint;not null;default:1;comment:其它转账通知 1启动 0禁止"`
	CreatedAt   time.Time `gorm:"autoCreateTime;type:timestamp;not null;comment:创建时间"`
	UpdatedAt   time.Time `gorm:"autoUpdateTime;type:timestamp;not null;comment:更新时间"`
}

// 启动时添加初始钱包地址
func addStartWalletAddress() {
	var _wa WalletAddress
	addresses := config.GetInitWalletAddress()
	
	if len(addresses) == 0 {
		fmt.Println("ℹ️  未配置WALLET_ADDRESS环境变量")
		return
	}

	fmt.Printf("🔍 开始处理 %d 个钱包地址配置...\n", len(addresses))

	for i, address := range addresses {
		address = strings.TrimSpace(address)
		if address == "" {
			continue
		}

		fmt.Printf("📝 处理地址 %d/%d: %s\n", i+1, len(addresses), address)

		// 验证地址格式
		isValid := help.IsValidTRONWalletAddress(address) || 
			help.IsValidPOLWalletAddress(address) || 
			help.IsValidOPTWalletAddress(address) || 
			help.IsValidBSCWalletAddress(address) || 
			help.IsValidARBWalletAddress(address) || 
			help.IsValidXLAYERWalletAddress(address) || 
			help.IsValidSOLWalletAddress(address) || 
			help.IsValidAPTWalletAddress(address)

		if !isValid {
			fmt.Printf("❌ 地址格式无效: %s\n", address)
			fmt.Println("   支持的格式:")
			fmt.Println("   - TRON:TxxxxxxxxxxxxxxxxxxxxxxxxxxxxR")
			fmt.Println("   - POLY:0x1234567890123456789012345678901234567890")
			fmt.Println("   - BSC:0x1234567890123456789012345678901234567890")
			fmt.Println("   - OP:0x1234567890123456789012345678901234567890")
			fmt.Println("   - ARB:0x1234567890123456789012345678901234567890")
			fmt.Println("   - XLAYER:0x1234567890123456789012345678901234567890")
			fmt.Println("   - SOL:1234567890123456789012345678901234567890123")
			fmt.Println("   - APT:0x1234567890123456789012345678901234567890123456789012345678901234")
			continue
		}

		// 解析地址
		_addresses := strings.Split(address, ":")
		if len(_addresses) != 2 {
			fmt.Printf("❌ 地址格式错误，缺少冒号分隔符: %s\n", address)
			continue
		}

		chain := _addresses[0]
		addr := _addresses[1]

		// 检查是否已存在
		var _res2 = DB.Where("chain = ? and address = ?", chain, addr).First(&_wa)
		if !errors.Is(_res2.Error, gorm.ErrRecordNotFound) {
			if _res2.Error == nil {
				fmt.Printf("ℹ️  地址已存在: %s:%s\n", chain, addr)
			} else {
				fmt.Printf("❌ 查询地址时出错: %v\n", _res2.Error)
			}
			continue
		}

		// 添加新地址
		var _row = WalletAddress{
			Chain:   chain,
			Address: addr,
			Status:  StatusEnable,
		}
		var _res = DB.Create(&_row)
		if _res.Error != nil {
			fmt.Printf("❌ 添加地址失败: %s - %v\n", address, _res.Error)
		} else if _res.RowsAffected == 1 {
			fmt.Printf("✅ 钱包地址添加成功: %s\n", address)
		} else {
			fmt.Printf("⚠️  地址添加异常: %s - 影响行数: %d\n", address, _res.RowsAffected)
		}
	}

	fmt.Println("🏁 钱包地址处理完成")
}

func (wa *WalletAddress) TableName() string {
	return "wallet_address"
}

func (wa *WalletAddress) SetStatus(status int) {
	wa.Status = status
	DB.Save(wa)
}

func (wa *WalletAddress) SetOtherNotify(notify int) {
	wa.OtherNotify = notify
	DB.Save(wa)
}

func (wa *WalletAddress) Delete() {
	DB.Delete(wa)
}

/*
判断是否存在
*/
func ExistsAddress(chain string, address string) bool {
	var rows []WalletAddress
	DB.Where("chain = ? and address = ? and status = ?", chain, address, StatusEnable).Find(&rows)
	return len(rows) > 0
}

func GetAvailableAddress(chain string) []WalletAddress {
	var rows []WalletAddress
	DB.Where("chain = ? and status = ?", chain, StatusEnable).Find(&rows)
	return rows
}

/*
获取所有链路的钱包地址统计信息
*/
func GetWalletAddressStats() map[string]int {
	var results []struct {
		Chain string
		Count int64
	}
	
	DB.Model(&WalletAddress{}).
		Select("chain, count(*) as count").
		Where("status = ?", StatusEnable).
		Group("chain").
		Find(&results)
	
	stats := make(map[string]int)
	for _, result := range results {
		stats[result.Chain] = int(result.Count)
	}
	
	return stats
}

func GetOtherNotify(chain string, address string) bool {
	var row WalletAddress
	var res = DB.Where("status = ? and chain = ? and address = ?", StatusEnable, chain, address).First(&row)
	if res.Error != nil {
		return false
	}
	return row.OtherNotify == 1
}
