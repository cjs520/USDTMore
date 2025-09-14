package model

import (
	"time"
)

type NotifyRecord struct {
	Txid      string    `gorm:"primaryKey;type:char(66);not null;comment:交易哈希;index:idx_notify_record_txid"`
	CreatedAt time.Time `gorm:"autoCreateTime;type:timestamptz;not null;comment:创建时间;index:idx_notify_record_created_at"`
	UpdatedAt time.Time `gorm:"autoUpdateTime;type:timestamptz;not null;comment:更新时间"`
}

func (nr *NotifyRecord) TableName() string {

	return "notify_record"
}

func IsNeedNotifyByTxid(txid string) bool {
	var row NotifyRecord
	var res = DB.Where("txid = ?", txid).Limit(1).Find(&row)
	if res.RowsAffected > 0 {

		return false
	}

	var row2 TradeOrders
	var res2 = DB.Where("trade_hash = ?", txid).Limit(1).Find(&row2)
	if res2.RowsAffected > 0 {

		return false
	}

	return true
}
