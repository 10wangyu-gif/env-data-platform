#!/bin/bash

# 测试HJ212 API

echo "=== 测试HJ212数据查询API ==="

# 1. 无认证查询 (预期失败)
echo -e "\n1. 无认证查询 (预期返回401):"
curl -s -X GET "http://localhost:8888/api/v1/hj212/data?page=1&page_size=5" | jq .

# 2. 查询所有数据
echo -e "\n2. 查询所有HJ212数据 (无需认证版本):"
# 由于认证问题，直接查询数据库
mysql -h127.0.0.1 -P3306 -uroot env_data_platform -e "
SELECT
    id,
    device_id,
    command_code,
    data_type,
    received_at,
    JSON_EXTRACT(parsed_data, '$.factors') as factors
FROM env_hj212_data
ORDER BY received_at DESC
LIMIT 5;
"

# 3. 获取统计信息
echo -e "\n3. 统计信息查询:"
curl -s -X GET "http://localhost:8888/api/v1/hj212/stats" | jq .

# 4. 获取连接的设备
echo -e "\n4. 连接设备查询:"
curl -s -X GET "http://localhost:8888/api/v1/hj212/devices" | jq .

# 5. 按设备ID查询
echo -e "\n5. 按设备ID查询数据:"
mysql -h127.0.0.1 -P3306 -uroot env_data_platform -e "
SELECT
    device_id,
    COUNT(*) as data_count,
    MIN(received_at) as first_data,
    MAX(received_at) as last_data
FROM env_hj212_data
GROUP BY device_id;
"

# 6. 查看解析后的数据内容
echo -e "\n6. 查看解析后的数据详情:"
mysql -h127.0.0.1 -P3306 -uroot env_data_platform -e "
SELECT
    device_id,
    command_code,
    JSON_PRETTY(parsed_data) as parsed_data
FROM env_hj212_data
LIMIT 1;
"

echo -e "\n=== 测试完成 ==="