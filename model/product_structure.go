package model

// ProductInfo is the normalized product record loaded from the current Apple Store catalog.
type ProductInfo struct {
	Model        string // 型号
	Capacity     string // 容量或尺寸
	Color        string // 颜色或手表款式
	Code         string // Apple 零件号
	Type         string // 产品类型标识
	PurchasePath string // Apple Store 官方选配页路径（手表使用）
}
