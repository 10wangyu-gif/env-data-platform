package services

import (
	"context"
	"testing"

	"github.com/env-data-platform/internal/models"
	"github.com/stretchr/testify/assert"
)

func TestMetadataSyncService_Basic(t *testing.T) {
	service := NewMetadataSyncService()

	// 测试HJ212元数据同步（这个应该能成功，因为是静态数据）
	t.Run("HJ212元数据同步", func(t *testing.T) {
		dataSource := &models.DataSource{
			Type: "hj212",
			ConfigData: []byte(`{
				"host": "192.168.1.101",
				"port": 9001,
				"device_id": "HJ212001"
			}`),
		}

		ctx := context.Background()
		result := service.SyncMetadata(ctx, dataSource)

		assert.True(t, result.Success, "HJ212元数据同步应该成功")
		assert.NotEmpty(t, result.Message, "消息不应为空")
		assert.Len(t, result.Tables, 1, "应该有一个HJ212数据表")

		if len(result.Tables) > 0 {
			table := result.Tables[0]
			assert.Equal(t, "hj212_realtime_data", table.Name, "表名应该正确")
			assert.Equal(t, 17, len(table.Columns), "字段数量应该正确")

			// 检查必要的字段
			columnNames := make(map[string]bool)
			for _, col := range table.Columns {
				columnNames[col.Name] = true
				assert.NotEmpty(t, col.Type, "字段类型不应为空")
				assert.NotEmpty(t, col.Comment, "字段注释不应为空")
			}

			// 验证关键字段存在
			expectedFields := []string{"QN", "ST", "CN", "MN", "DataTime", "a21026", "a21004"}
			for _, field := range expectedFields {
				assert.True(t, columnNames[field], "应该包含字段: %s", field)
			}
		}

		// 检查基本字段
		assert.NotNil(t, result.SyncedAt, "同步时间不应为空")
		assert.NotNil(t, result.Details, "详情不应为空")
		assert.Contains(t, result.Details, "field_count", "应该包含字段数量信息")
		assert.Equal(t, "HJ212001", result.Details["device_id"], "设备ID应该匹配")
	})

	// 测试MySQL元数据同步（预期失败，因为没有数据库）
	t.Run("MySQL元数据同步", func(t *testing.T) {
		dataSource := &models.DataSource{
			Type: "mysql",
			ConfigData: []byte(`{
				"host": "localhost",
				"port": 3306,
				"username": "root",
				"password": "password",
				"database": "test"
			}`),
		}

		ctx := context.Background()
		result := service.SyncMetadata(ctx, dataSource)

		// 由于本地没有MySQL实例，预期失败
		assert.False(t, result.Success, "预期同步失败但实际成功")
		assert.NotEmpty(t, result.Message, "错误消息不应为空")
		assert.NotNil(t, result.SyncedAt, "同步时间不应为空")
		assert.NotNil(t, result.Details, "详情不应为空")
	})

	// 测试PostgreSQL元数据同步（预期失败，因为没有数据库）
	t.Run("PostgreSQL元数据同步", func(t *testing.T) {
		dataSource := &models.DataSource{
			Type: "postgresql",
			ConfigData: []byte(`{
				"host": "localhost",
				"port": 5432,
				"username": "postgres",
				"password": "password",
				"database": "test"
			}`),
		}

		ctx := context.Background()
		result := service.SyncMetadata(ctx, dataSource)

		// 由于本地没有PostgreSQL实例，预期失败
		assert.False(t, result.Success, "预期同步失败但实际成功")
		assert.NotEmpty(t, result.Message, "错误消息不应为空")
		assert.NotNil(t, result.SyncedAt, "同步时间不应为空")
		assert.NotNil(t, result.Details, "详情不应为空")
	})

	// 测试不支持的类型
	t.Run("不支持的类型", func(t *testing.T) {
		dataSource := &models.DataSource{
			Type: "unsupported_type",
			ConfigData: []byte(`{
				"some_config": "value"
			}`),
		}

		ctx := context.Background()
		result := service.SyncMetadata(ctx, dataSource)

		assert.False(t, result.Success, "不支持的类型应该失败")
		assert.Contains(t, result.Message, "不支持的数据源类型", "错误消息应该提及不支持的类型")
		assert.NotNil(t, result.SyncedAt, "同步时间不应为空")
		assert.NotNil(t, result.Details, "详情不应为空")
		assert.Len(t, result.Tables, 0, "不应该有表信息")
	})

	// 测试配置解析错误
	t.Run("配置解析错误", func(t *testing.T) {
		dataSource := &models.DataSource{
			Type:       "hj212",
			ConfigData: []byte(`invalid json`),
		}

		ctx := context.Background()
		result := service.SyncMetadata(ctx, dataSource)

		assert.False(t, result.Success, "无效配置应该失败")
		assert.Contains(t, result.Message, "解析配置失败", "错误消息应该提及配置解析失败")
	})
}

func TestTableMetadata_Structure(t *testing.T) {
	// 创建一个示例表元数据
	table := TableMetadata{
		Name:     "test_table",
		Schema:   "public",
		Comment:  "测试表",
		RowCount: 1000,
		Size:     1024 * 1024, // 1MB
		Columns: []ColumnMetadata{
			{
				Name:         "id",
				Type:         "int",
				IsNullable:   false,
				IsPrimaryKey: true,
				IsAutoIncr:   true,
				Comment:      "主键ID",
			},
			{
				Name:         "name",
				Type:         "varchar",
				Length:       &[]int{255}[0],
				IsNullable:   false,
				IsPrimaryKey: false,
				DefaultValue: "",
				Comment:      "名称",
			},
		},
	}

	// 验证表结构
	assert.Equal(t, "test_table", table.Name)
	assert.Equal(t, "public", table.Schema)
	assert.Equal(t, "测试表", table.Comment)
	assert.Equal(t, int64(1000), table.RowCount)
	assert.Equal(t, int64(1024*1024), table.Size)

	// 验证列结构
	assert.Len(t, table.Columns, 2)

	// 验证主键列
	idCol := table.Columns[0]
	assert.Equal(t, "id", idCol.Name)
	assert.Equal(t, "int", idCol.Type)
	assert.False(t, idCol.IsNullable)
	assert.True(t, idCol.IsPrimaryKey)
	assert.True(t, idCol.IsAutoIncr)

	// 验证varchar列
	nameCol := table.Columns[1]
	assert.Equal(t, "name", nameCol.Name)
	assert.Equal(t, "varchar", nameCol.Type)
	assert.NotNil(t, nameCol.Length)
	assert.Equal(t, 255, *nameCol.Length)
}