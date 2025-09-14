package help

import (
	"errors"
	"fmt"
	"math"
	"strconv"

	"github.com/shopspring/decimal"
)

// DecimalPrecision 定义默认的decimal精度常量
const (
	// MoneyPrecision 货币精度（人民币等法币）
	MoneyPrecision = 2
	// CryptoPrecision 加密货币精度（USDT等）
	CryptoPrecision = 8
	// RatePrecision 汇率精度
	RatePrecision = 8
)

// SafeDecimalFromFloat 安全地将float64转换为decimal.Decimal
// 避免浮点数精度丢失问题
func SafeDecimalFromFloat(value float64) (decimal.Decimal, error) {
	if math.IsNaN(value) {
		return decimal.Zero, errors.New("value is NaN")
	}
	if math.IsInf(value, 0) {
		return decimal.Zero, errors.New("value is infinite")
	}
	
	// 使用字符串转换避免浮点数精度问题
	str := strconv.FormatFloat(value, 'f', -1, 64)
	return decimal.NewFromString(str)
}

// SafeDecimalFromString 安全地将字符串转换为decimal.Decimal
func SafeDecimalFromString(value string) (decimal.Decimal, error) {
	if value == "" {
		return decimal.Zero, errors.New("empty string")
	}
	return decimal.NewFromString(value)
}

// FormatMoney 格式化货币金额（保留2位小数）
func FormatMoney(amount decimal.Decimal) string {
	return amount.StringFixed(MoneyPrecision)
}

// FormatCrypto 格式化加密货币金额（保留8位小数，去除尾随零）
func FormatCrypto(amount decimal.Decimal) string {
	// 移除尾随的零
	return amount.String()
}

// FormatCryptoFixed 格式化加密货币金额（固定2位小数，用于订单匹配）
func FormatCryptoFixed(amount decimal.Decimal) string {
	return amount.StringFixed(MoneyPrecision)
}

// ValidateAmount 验证金额是否有效
func ValidateAmount(amount decimal.Decimal) error {
	if amount.IsNegative() {
		return errors.New("amount cannot be negative")
	}
	if amount.IsZero() {
		return errors.New("amount cannot be zero")
	}
	// 检查是否超过合理范围（1000万）
	maxAmount := decimal.NewFromFloat(10000000)
	if amount.GreaterThan(maxAmount) {
		return errors.New("amount exceeds maximum limit")
	}
	return nil
}

// ValidateMoneyAmount 验证货币金额（法币）
func ValidateMoneyAmount(amount decimal.Decimal) error {
	if err := ValidateAmount(amount); err != nil {
		return err
	}
	// 检查小数位数不超过2位
	if amount.Exponent() < -MoneyPrecision {
		return errors.New("money amount has too many decimal places")
	}
	return nil
}

// ValidateCryptoAmount 验证加密货币金额
func ValidateCryptoAmount(amount decimal.Decimal) error {
	if err := ValidateAmount(amount); err != nil {
		return err
	}
	// 检查小数位数不超过8位
	if amount.Exponent() < -CryptoPrecision {
		return errors.New("crypto amount has too many decimal places")
	}
	return nil
}

// CalculateUSDTAmount 计算USDT金额
// money: 法币金额, rate: USDT汇率 (1 USDT = rate CNY)
func CalculateUSDTAmount(money, rate decimal.Decimal) (decimal.Decimal, error) {
	if rate.IsZero() {
		return decimal.Zero, errors.New("rate cannot be zero")
	}
	if err := ValidateMoneyAmount(money); err != nil {
		return decimal.Zero, fmt.Errorf("invalid money amount: %w", err)
	}
	
	usdtAmount := money.Div(rate)
	return usdtAmount, nil
}

// CalculateMoneyAmount 计算法币金额
// usdt: USDT金额, rate: USDT汇率 (1 USDT = rate CNY)  
func CalculateMoneyAmount(usdt, rate decimal.Decimal) (decimal.Decimal, error) {
	if err := ValidateCryptoAmount(usdt); err != nil {
		return decimal.Zero, fmt.Errorf("invalid USDT amount: %w", err)
	}
	if rate.IsZero() {
		return decimal.Zero, errors.New("rate cannot be zero")
	}
	
	moneyAmount := usdt.Mul(rate)
	return moneyAmount, nil
}

// CompareAmounts 比较两个金额是否相等（考虑精度误差）
func CompareAmounts(a, b decimal.Decimal, precision int32) bool {
	// 将两个数都四舍五入到指定精度后比较
	aRounded := a.Round(precision)
	bRounded := b.Round(precision)
	return aRounded.Equal(bRounded)
}

// IsInPaymentRange 检查金额是否在有效支付范围内
func IsInPaymentRange(amount decimal.Decimal) bool {
	minAmount := decimal.NewFromFloat(0.01) // 最小0.01 USDT
	maxAmount := decimal.NewFromFloat(100000) // 最大100,000 USDT
	
	return amount.GreaterThanOrEqual(minAmount) && amount.LessThanOrEqual(maxAmount)
}

// AddAtomicIncrement 添加原子增量
func AddAtomicIncrement(baseAmount decimal.Decimal, increment int, atomicity decimal.Decimal) decimal.Decimal {
	incrementAmount := atomicity.Mul(decimal.NewFromInt(int64(increment)))
	return baseAmount.Add(incrementAmount)
}

// ParseDecimalFromInterface 从interface{}安全解析decimal
func ParseDecimalFromInterface(value interface{}) (decimal.Decimal, error) {
	switch v := value.(type) {
	case decimal.Decimal:
		return v, nil
	case float64:
		return SafeDecimalFromFloat(v)
	case float32:
		return SafeDecimalFromFloat(float64(v))
	case int:
		return decimal.NewFromInt(int64(v)), nil
	case int32:
		return decimal.NewFromInt(int64(v)), nil
	case int64:
		return decimal.NewFromInt(v), nil
	case string:
		return SafeDecimalFromString(v)
	default:
		return decimal.Zero, fmt.Errorf("cannot convert %T to decimal", value)
	}
}

// ToFloat64Safe 安全地将decimal转换为float64（用于向后兼容）
func ToFloat64Safe(d decimal.Decimal) (float64, error) {
	f, exact := d.Float64()
	if !exact {
		return f, fmt.Errorf("precision loss when converting decimal %s to float64", d.String())
	}
	return f, nil
}

// RoundToAtomicity 将金额四舍五入到指定原子精度
func RoundToAtomicity(amount decimal.Decimal, atomicity decimal.Decimal) decimal.Decimal {
	// 计算需要的倍数
	multiplier := amount.Div(atomicity)
	// 四舍五入到最近的整数
	rounded := multiplier.Round(0)
	// 乘回原子精度
	return rounded.Mul(atomicity)
}

// MarshalJSON 自定义JSON序列化，确保decimal在JSON中显示为字符串
func DecimalToJSONString(d decimal.Decimal) string {
	return d.String()
}

// UnmarshalDecimalFromJSON 从JSON字符串反序列化decimal
func UnmarshalDecimalFromJSON(jsonValue interface{}) (decimal.Decimal, error) {
	return ParseDecimalFromInterface(jsonValue)
}

// CompareDecimalAmounts 比较两个decimal金额是否相等（考虑原子精度）
func CompareDecimalAmounts(a, b decimal.Decimal) bool {
	return CompareAmounts(a, b, MoneyPrecision)
}

// ValidateDecimalRange 验证decimal值在指定范围内
func ValidateDecimalRange(value, min, max decimal.Decimal) error {
	if value.LessThan(min) {
		return fmt.Errorf("value %s is less than minimum %s", value.String(), min.String())
	}
	if value.GreaterThan(max) {
		return fmt.Errorf("value %s is greater than maximum %s", value.String(), max.String())
	}
	return nil
}

// NormalizeDecimalPrecision 标准化decimal精度
func NormalizeDecimalPrecision(value decimal.Decimal, precision int32) decimal.Decimal {
	return value.Round(precision)
}

// AdjustAmountForAtomicity 根据原子精度调整金额
func AdjustAmountForAtomicity(amount decimal.Decimal, atomicity decimal.Decimal) decimal.Decimal {
	return RoundToAtomicity(amount, atomicity)
}

// CalculatePercentage 计算百分比
func CalculatePercentage(value, total decimal.Decimal) (decimal.Decimal, error) {
	if total.IsZero() {
		return decimal.Zero, errors.New("total cannot be zero")
	}
	hundred := decimal.NewFromInt(100)
	return value.Div(total).Mul(hundred), nil
}

// ApplyPercentage 应用百分比到数值
func ApplyPercentage(base decimal.Decimal, percentage decimal.Decimal) decimal.Decimal {
	hundred := decimal.NewFromInt(100)
	return base.Mul(percentage).Div(hundred)
}

// IsDecimalEqual 检查两个decimal是否完全相等
func IsDecimalEqual(a, b decimal.Decimal) bool {
	return a.Equal(b)
}

// MinDecimal 返回两个decimal中的最小值
func MinDecimal(a, b decimal.Decimal) decimal.Decimal {
	if a.LessThan(b) {
		return a
	}
	return b
}

// MaxDecimal 返回两个decimal中的最大值
func MaxDecimal(a, b decimal.Decimal) decimal.Decimal {
	if a.GreaterThan(b) {
		return a
	}
	return b
}

// SumDecimals 计算decimal切片的总和
func SumDecimals(values []decimal.Decimal) decimal.Decimal {
	sum := decimal.Zero
	for _, value := range values {
		sum = sum.Add(value)
	}
	return sum
}

// AverageDecimals 计算decimal切片的平均值
func AverageDecimals(values []decimal.Decimal) (decimal.Decimal, error) {
	if len(values) == 0 {
		return decimal.Zero, errors.New("empty values slice")
	}
	
	sum := SumDecimals(values)
	count := decimal.NewFromInt(int64(len(values)))
	return sum.Div(count), nil
}

// ConvertDecimalToFixedPoint 将decimal转换为定点数表示（用于数据库存储优化）
func ConvertDecimalToFixedPoint(d decimal.Decimal, scale int32) (int64, error) {
	scaleFactor := decimal.New(1, scale)
	scaled := d.Mul(scaleFactor)
	
	// 检查是否在int64范围内
	if scaled.GreaterThan(decimal.NewFromInt(9223372036854775807)) || 
	   scaled.LessThan(decimal.NewFromInt(-9223372036854775808)) {
		return 0, errors.New("value exceeds int64 range")
	}
	
	return scaled.IntPart(), nil
}

// ConvertFixedPointToDecimal 将定点数转换回decimal
func ConvertFixedPointToDecimal(value int64, scale int32) decimal.Decimal {
	d := decimal.NewFromInt(value)
	scaleFactor := decimal.New(1, scale)
	return d.Div(scaleFactor)
}