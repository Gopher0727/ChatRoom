# 数据模型说明

## 模型设计

本项目的数据模型不使用 `gorm.Model`，而是手动定义字段。

### 为什么不使用 gorm.Model？

`gorm.Model` 的定义：
```go
type Model struct {
    ID        uint           `gorm:"primaryKey"`
    CreatedAt time.Time
    UpdatedAt time.Time
    DeletedAt gorm.DeletedAt `gorm:"index"`
}
```

1. `gorm.Model` 的 ID 是 `uint` 类型（自增整数）
2. 使用 Snowflake 算法生成分布式 ID，类型是 `string`
3. 不需要软删除功能（`DeletedAt`）
