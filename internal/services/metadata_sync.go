package services

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/env-data-platform/internal/models"
	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
)

// MetadataSyncService 元数据同步服务
type MetadataSyncService struct{}

// NewMetadataSyncService 创建元数据同步服务实例
func NewMetadataSyncService() *MetadataSyncService {
	return &MetadataSyncService{}
}

// MetadataSyncResult 元数据同步结果
type MetadataSyncResult struct {
	Success   bool                   `json:"success"`
	Message   string                 `json:"message"`
	Tables    []TableMetadata        `json:"tables"`
	Details   map[string]interface{} `json:"details,omitempty"`
	SyncedAt  time.Time              `json:"synced_at"`
}

// TableMetadata 表元数据
type TableMetadata struct {
	Name        string           `json:"name"`
	Schema      string           `json:"schema,omitempty"`
	Comment     string           `json:"comment,omitempty"`
	RowCount    int64            `json:"row_count"`
	Size        int64            `json:"size_bytes"`
	Columns     []ColumnMetadata `json:"columns"`
	Indexes     []IndexMetadata  `json:"indexes,omitempty"`
	UpdatedAt   *time.Time       `json:"updated_at,omitempty"`
}

// ColumnMetadata 列元数据
type ColumnMetadata struct {
	Name         string `json:"name"`
	Type         string `json:"type"`
	Length       *int   `json:"length,omitempty"`
	Precision    *int   `json:"precision,omitempty"`
	Scale        *int   `json:"scale,omitempty"`
	IsNullable   bool   `json:"is_nullable"`
	IsPrimaryKey bool   `json:"is_primary_key"`
	IsAutoIncr   bool   `json:"is_auto_increment,omitempty"`
	DefaultValue string `json:"default_value,omitempty"`
	Comment      string `json:"comment,omitempty"`
}

// IndexMetadata 索引元数据
type IndexMetadata struct {
	Name     string   `json:"name"`
	Type     string   `json:"type"`
	Columns  []string `json:"columns"`
	IsUnique bool     `json:"is_unique"`
}

// SyncMetadata 同步数据源元数据
func (s *MetadataSyncService) SyncMetadata(ctx context.Context, dataSource *models.DataSource) *MetadataSyncResult {
	startTime := time.Now()

	result := &MetadataSyncResult{
		SyncedAt: startTime,
		Details:  make(map[string]interface{}),
	}

	switch dataSource.Type {
	case "mysql":
		result = s.syncMySQLMetadata(ctx, dataSource, result)
	case "postgresql":
		result = s.syncPostgreSQLMetadata(ctx, dataSource, result)
	case "hj212":
		result = s.syncHJ212Metadata(ctx, dataSource, result)
	default:
		result.Success = false
		result.Message = fmt.Sprintf("不支持的数据源类型: %s", dataSource.Type)
	}

	return result
}

// syncMySQLMetadata 同步MySQL元数据
func (s *MetadataSyncService) syncMySQLMetadata(ctx context.Context, dataSource *models.DataSource, result *MetadataSyncResult) *MetadataSyncResult {
	// 解析配置
	var config map[string]interface{}
	if err := json.Unmarshal(dataSource.ConfigData, &config); err != nil {
		result.Success = false
		result.Message = fmt.Sprintf("解析配置失败: %v", err)
		return result
	}

	// 构建MySQL连接字符串
	host := config["host"].(string)
	port := int(config["port"].(float64))
	username := config["username"].(string)
	password := config["password"].(string)
	database := config["database"].(string)

	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?parseTime=true&charset=utf8mb4",
		username, password, host, port, database)

	// 连接数据库
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		result.Success = false
		result.Message = fmt.Sprintf("MySQL连接失败: %v", err)
		return result
	}
	defer db.Close()

	// 设置查询超时
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// 验证连接
	if err := db.PingContext(ctx); err != nil {
		result.Success = false
		result.Message = fmt.Sprintf("MySQL ping失败: %v", err)
		return result
	}

	// 获取表列表
	tables, err := s.getMySQLTables(ctx, db, database)
	if err != nil {
		result.Success = false
		result.Message = fmt.Sprintf("获取表列表失败: %v", err)
		return result
	}

	// 获取每个表的详细信息
	var tableMetadata []TableMetadata
	for _, tableName := range tables {
		metadata, err := s.getMySQLTableMetadata(ctx, db, database, tableName)
		if err != nil {
			continue // 跳过获取失败的表
		}
		tableMetadata = append(tableMetadata, *metadata)
	}

	result.Success = true
	result.Message = fmt.Sprintf("成功同步 %d 个表的元数据", len(tableMetadata))
	result.Tables = tableMetadata
	result.Details["database"] = database
	result.Details["table_count"] = len(tableMetadata)

	return result
}

// getMySQLTables 获取MySQL表列表
func (s *MetadataSyncService) getMySQLTables(ctx context.Context, db *sql.DB, database string) ([]string, error) {
	query := `
		SELECT table_name
		FROM information_schema.tables
		WHERE table_schema = ? AND table_type = 'BASE TABLE'
		ORDER BY table_name
	`

	rows, err := db.QueryContext(ctx, query, database)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var tableName string
		if err := rows.Scan(&tableName); err != nil {
			continue
		}
		tables = append(tables, tableName)
	}

	return tables, nil
}

// getMySQLTableMetadata 获取MySQL表的详细元数据
func (s *MetadataSyncService) getMySQLTableMetadata(ctx context.Context, db *sql.DB, database, tableName string) (*TableMetadata, error) {
	metadata := &TableMetadata{
		Name:   tableName,
		Schema: database,
	}

	// 获取表注释和行数
	tableInfoQuery := `
		SELECT table_comment, table_rows,
			   ROUND(((data_length + index_length) / 1024 / 1024), 2) AS size_mb
		FROM information_schema.tables
		WHERE table_schema = ? AND table_name = ?
	`

	var sizeMB float64
	err := db.QueryRowContext(ctx, tableInfoQuery, database, tableName).Scan(
		&metadata.Comment, &metadata.RowCount, &sizeMB)
	if err != nil {
		return nil, err
	}
	metadata.Size = int64(sizeMB * 1024 * 1024) // 转换为字节

	// 获取列信息
	columns, err := s.getMySQLColumns(ctx, db, database, tableName)
	if err != nil {
		return nil, err
	}
	metadata.Columns = columns

	// 获取索引信息
	indexes, err := s.getMySQLIndexes(ctx, db, database, tableName)
	if err == nil {
		metadata.Indexes = indexes
	}

	return metadata, nil
}

// getMySQLColumns 获取MySQL表的列信息
func (s *MetadataSyncService) getMySQLColumns(ctx context.Context, db *sql.DB, database, tableName string) ([]ColumnMetadata, error) {
	query := `
		SELECT column_name, data_type, is_nullable, column_key,
			   column_default, extra, column_comment,
			   character_maximum_length, numeric_precision, numeric_scale
		FROM information_schema.columns
		WHERE table_schema = ? AND table_name = ?
		ORDER BY ordinal_position
	`

	rows, err := db.QueryContext(ctx, query, database, tableName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var columns []ColumnMetadata
	for rows.Next() {
		var col ColumnMetadata
		var isNullable, columnKey, extra string
		var defaultValue, comment sql.NullString
		var maxLength, precision, scale sql.NullInt64

		err := rows.Scan(
			&col.Name, &col.Type, &isNullable, &columnKey,
			&defaultValue, &extra, &comment,
			&maxLength, &precision, &scale,
		)
		if err != nil {
			continue
		}

		col.IsNullable = (isNullable == "YES")
		col.IsPrimaryKey = (columnKey == "PRI")
		col.IsAutoIncr = (extra == "auto_increment")

		if defaultValue.Valid {
			col.DefaultValue = defaultValue.String
		}
		if comment.Valid {
			col.Comment = comment.String
		}
		if maxLength.Valid {
			length := int(maxLength.Int64)
			col.Length = &length
		}
		if precision.Valid {
			prec := int(precision.Int64)
			col.Precision = &prec
		}
		if scale.Valid {
			sc := int(scale.Int64)
			col.Scale = &sc
		}

		columns = append(columns, col)
	}

	return columns, nil
}

// getMySQLIndexes 获取MySQL表的索引信息
func (s *MetadataSyncService) getMySQLIndexes(ctx context.Context, db *sql.DB, database, tableName string) ([]IndexMetadata, error) {
	query := `
		SELECT index_name, index_type, column_name, non_unique
		FROM information_schema.statistics
		WHERE table_schema = ? AND table_name = ?
		ORDER BY index_name, seq_in_index
	`

	rows, err := db.QueryContext(ctx, query, database, tableName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	indexMap := make(map[string]*IndexMetadata)
	for rows.Next() {
		var indexName, indexType, columnName string
		var nonUnique int

		if err := rows.Scan(&indexName, &indexType, &columnName, &nonUnique); err != nil {
			continue
		}

		if index, exists := indexMap[indexName]; exists {
			index.Columns = append(index.Columns, columnName)
		} else {
			indexMap[indexName] = &IndexMetadata{
				Name:     indexName,
				Type:     indexType,
				Columns:  []string{columnName},
				IsUnique: (nonUnique == 0),
			}
		}
	}

	var indexes []IndexMetadata
	for _, index := range indexMap {
		indexes = append(indexes, *index)
	}

	return indexes, nil
}

// syncPostgreSQLMetadata 同步PostgreSQL元数据
func (s *MetadataSyncService) syncPostgreSQLMetadata(ctx context.Context, dataSource *models.DataSource, result *MetadataSyncResult) *MetadataSyncResult {
	// 解析配置
	var config map[string]interface{}
	if err := json.Unmarshal(dataSource.ConfigData, &config); err != nil {
		result.Success = false
		result.Message = fmt.Sprintf("解析配置失败: %v", err)
		return result
	}

	// 构建PostgreSQL连接字符串
	host := config["host"].(string)
	port := int(config["port"].(float64))
	username := config["username"].(string)
	password := config["password"].(string)
	database := config["database"].(string)

	dsn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		host, port, username, password, database)

	// 连接数据库
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		result.Success = false
		result.Message = fmt.Sprintf("PostgreSQL连接失败: %v", err)
		return result
	}
	defer db.Close()

	// 设置查询超时
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// 验证连接
	if err := db.PingContext(ctx); err != nil {
		result.Success = false
		result.Message = fmt.Sprintf("PostgreSQL ping失败: %v", err)
		return result
	}

	// 获取表列表
	tables, err := s.getPostgreSQLTables(ctx, db)
	if err != nil {
		result.Success = false
		result.Message = fmt.Sprintf("获取表列表失败: %v", err)
		return result
	}

	// 获取每个表的详细信息
	var tableMetadata []TableMetadata
	for _, tableName := range tables {
		metadata, err := s.getPostgreSQLTableMetadata(ctx, db, tableName)
		if err != nil {
			continue // 跳过获取失败的表
		}
		tableMetadata = append(tableMetadata, *metadata)
	}

	result.Success = true
	result.Message = fmt.Sprintf("成功同步 %d 个表的元数据", len(tableMetadata))
	result.Tables = tableMetadata
	result.Details["database"] = database
	result.Details["table_count"] = len(tableMetadata)

	return result
}

// getPostgreSQLTables 获取PostgreSQL表列表
func (s *MetadataSyncService) getPostgreSQLTables(ctx context.Context, db *sql.DB) ([]string, error) {
	query := `
		SELECT tablename
		FROM pg_tables
		WHERE schemaname = 'public'
		ORDER BY tablename
	`

	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var tableName string
		if err := rows.Scan(&tableName); err != nil {
			continue
		}
		tables = append(tables, tableName)
	}

	return tables, nil
}

// getPostgreSQLTableMetadata 获取PostgreSQL表的详细元数据
func (s *MetadataSyncService) getPostgreSQLTableMetadata(ctx context.Context, db *sql.DB, tableName string) (*TableMetadata, error) {
	metadata := &TableMetadata{
		Name:   tableName,
		Schema: "public",
	}

	// 获取表行数（近似值）
	rowCountQuery := `SELECT reltuples::bigint FROM pg_class WHERE relname = $1`
	db.QueryRowContext(ctx, rowCountQuery, tableName).Scan(&metadata.RowCount)

	// 获取表大小
	sizeQuery := `SELECT pg_total_relation_size($1)`
	db.QueryRowContext(ctx, sizeQuery, tableName).Scan(&metadata.Size)

	// 获取列信息
	columns, err := s.getPostgreSQLColumns(ctx, db, tableName)
	if err != nil {
		return nil, err
	}
	metadata.Columns = columns

	return metadata, nil
}

// getPostgreSQLColumns 获取PostgreSQL表的列信息
func (s *MetadataSyncService) getPostgreSQLColumns(ctx context.Context, db *sql.DB, tableName string) ([]ColumnMetadata, error) {
	query := `
		SELECT column_name, data_type, is_nullable, column_default,
			   character_maximum_length, numeric_precision, numeric_scale
		FROM information_schema.columns
		WHERE table_name = $1 AND table_schema = 'public'
		ORDER BY ordinal_position
	`

	rows, err := db.QueryContext(ctx, query, tableName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var columns []ColumnMetadata
	for rows.Next() {
		var col ColumnMetadata
		var isNullable string
		var defaultValue sql.NullString
		var maxLength, precision, scale sql.NullInt64

		err := rows.Scan(
			&col.Name, &col.Type, &isNullable, &defaultValue,
			&maxLength, &precision, &scale,
		)
		if err != nil {
			continue
		}

		col.IsNullable = (isNullable == "YES")

		if defaultValue.Valid {
			col.DefaultValue = defaultValue.String
		}
		if maxLength.Valid {
			length := int(maxLength.Int64)
			col.Length = &length
		}
		if precision.Valid {
			prec := int(precision.Int64)
			col.Precision = &prec
		}
		if scale.Valid {
			sc := int(scale.Int64)
			col.Scale = &sc
		}

		columns = append(columns, col)
	}

	return columns, nil
}

// syncHJ212Metadata 同步HJ212设备元数据
func (s *MetadataSyncService) syncHJ212Metadata(ctx context.Context, dataSource *models.DataSource, result *MetadataSyncResult) *MetadataSyncResult {
	// 解析配置
	var config map[string]interface{}
	if err := json.Unmarshal(dataSource.ConfigData, &config); err != nil {
		result.Success = false
		result.Message = fmt.Sprintf("解析配置失败: %v", err)
		return result
	}

	// 模拟HJ212设备的数据字段
	hj212Fields := []ColumnMetadata{
		{Name: "QN", Type: "string", Comment: "请求编号"},
		{Name: "ST", Type: "string", Comment: "系统编码"},
		{Name: "CN", Type: "string", Comment: "命令编码"},
		{Name: "PW", Type: "string", Comment: "访问密码"},
		{Name: "MN", Type: "string", Comment: "设备唯一标识"},
		{Name: "Flag", Type: "int", Comment: "标志位"},
		{Name: "CP", Type: "string", Comment: "命令参数"},
		{Name: "DataTime", Type: "datetime", Comment: "数据时间"},
		{Name: "a21026", Type: "float", Comment: "实时数据_二氧化硫"},
		{Name: "a21004", Type: "float", Comment: "实时数据_氮氧化物"},
		{Name: "a21005", Type: "float", Comment: "实时数据_一氧化碳"},
		{Name: "a05024", Type: "float", Comment: "实时数据_氧气"},
		{Name: "a21002", Type: "float", Comment: "实时数据_颗粒物"},
		{Name: "a19001", Type: "float", Comment: "实时数据_烟气温度"},
		{Name: "a19002", Type: "float", Comment: "实时数据_烟气压力"},
		{Name: "a19003", Type: "float", Comment: "实时数据_烟气流速"},
		{Name: "a19004", Type: "float", Comment: "实时数据_烟气湿度"},
	}

	hj212Table := TableMetadata{
		Name:    "hj212_realtime_data",
		Comment: "HJ212设备实时数据",
		Columns: hj212Fields,
	}

	if deviceId, ok := config["device_id"].(string); ok {
		result.Details["device_id"] = deviceId
	}

	result.Success = true
	result.Message = "成功获取HJ212设备数据字段映射"
	result.Tables = []TableMetadata{hj212Table}
	result.Details["field_count"] = len(hj212Fields)

	return result
}