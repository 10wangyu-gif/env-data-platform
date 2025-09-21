package services

import (
	"context"
	"testing"

	"github.com/env-data-platform/internal/models"
	"github.com/stretchr/testify/assert"
)

func TestConnectionTestService_Basic(t *testing.T) {
	service := NewConnectionTestService()

	// 测试MySQL连接
	t.Run("MySQL连接测试", func(t *testing.T) {
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
		result := service.TestConnection(ctx, dataSource)

		// 由于本地可能没有MySQL实例，预期失败
		assert.False(t, result.Success, "预期连接失败但实际成功")
		assert.NotEmpty(t, result.Message, "错误消息不应为空")
		assert.NotNil(t, result.TestedAt, "测试时间不应为空")
		assert.NotNil(t, result.Details, "详情不应为空")
		assert.GreaterOrEqual(t, result.Latency.Milliseconds(), int64(0), "延迟应该非负")
	})

	// 测试PostgreSQL连接
	t.Run("PostgreSQL连接测试", func(t *testing.T) {
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
		result := service.TestConnection(ctx, dataSource)

		// 由于本地可能没有PostgreSQL实例，预期失败
		assert.False(t, result.Success, "预期连接失败但实际成功")
		assert.NotEmpty(t, result.Message, "错误消息不应为空")
		assert.NotNil(t, result.TestedAt, "测试时间不应为空")
		assert.NotNil(t, result.Details, "详情不应为空")
		assert.GreaterOrEqual(t, result.Latency.Milliseconds(), int64(0), "延迟应该非负")
	})

	// 测试HJ212连接
	t.Run("HJ212连接测试", func(t *testing.T) {
		dataSource := &models.DataSource{
			Type: "hj212",
			ConfigData: []byte(`{
				"host": "localhost",
				"port": 9001,
				"device_id": "HJ212001"
			}`),
		}

		ctx := context.Background()
		result := service.TestConnection(ctx, dataSource)

		// 由于没有HJ212设备监听，预期失败
		assert.False(t, result.Success, "预期连接失败但实际成功")
		assert.NotEmpty(t, result.Message, "错误消息不应为空")
		assert.NotNil(t, result.TestedAt, "测试时间不应为空")
		assert.NotNil(t, result.Details, "详情不应为空")
		assert.GreaterOrEqual(t, result.Latency.Milliseconds(), int64(0), "延迟应该非负")

		// HJ212特定检查
		assert.Contains(t, result.Details, "device_id", "应该包含设备ID")
	})

	// 测试API连接
	t.Run("API连接测试", func(t *testing.T) {
		dataSource := &models.DataSource{
			Type: "api",
			ConfigData: []byte(`{
				"url": "https://httpbin.org/get",
				"method": "GET"
			}`),
		}

		ctx := context.Background()
		result := service.TestConnection(ctx, dataSource)

		// httpbin.org应该可以访问
		assert.True(t, result.Success, "预期连接成功但实际失败: %s", result.Message)
		assert.NotEmpty(t, result.Message, "消息不应为空")
		assert.NotNil(t, result.TestedAt, "测试时间不应为空")
		assert.NotNil(t, result.Details, "详情不应为空")
		assert.GreaterOrEqual(t, result.Latency.Milliseconds(), int64(0), "延迟应该非负")

		// API特定检查
		assert.Contains(t, result.Details, "status_code", "应该包含状态码")
		assert.Contains(t, result.Details, "status", "应该包含状态描述")
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
		result := service.TestConnection(ctx, dataSource)

		assert.False(t, result.Success, "不支持的类型应该失败")
		assert.Contains(t, result.Message, "不支持的数据源类型", "错误消息应该提及不支持的类型")
		assert.NotNil(t, result.TestedAt, "测试时间不应为空")
		assert.NotNil(t, result.Details, "详情不应为空")
	})

	// 测试配置解析错误
	t.Run("配置解析错误", func(t *testing.T) {
		dataSource := &models.DataSource{
			Type:       "mysql",
			ConfigData: []byte(`invalid json`),
		}

		ctx := context.Background()
		result := service.TestConnection(ctx, dataSource)

		assert.False(t, result.Success, "无效配置应该失败")
		assert.Contains(t, result.Message, "解析配置失败", "错误消息应该提及配置解析失败")
	})
}