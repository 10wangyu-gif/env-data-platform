#!/bin/bash

echo "=== 告警功能测试 ==="

# 使用nc (netcat) 发送测试数据
HOST="localhost"
PORT="8212"

echo -e "\n1. 测试PM2.5严重超标告警 (200.5 > 150阈值)"
echo "##0180QN=20250920183500001;ST=22;CN=2011;PW=123456;MN=ALARM_TEST_001;Flag=0;CP=&&DataTime=20250920183500,a34004-Rtd=200.5,a34004-Flag=N,a21001-Rtd=0.08,a21001-Flag=N&&1234" | nc $HOST $PORT

sleep 2

echo -e "\n2. 测试SO2轻微超标告警 (0.6 > 0.5阈值)"
echo "##0175QN=20250920183502002;ST=22;CN=2011;PW=123456;MN=ALARM_TEST_002;Flag=0;CP=&&DataTime=20250920183502,a21001-Rtd=0.6,a21001-Flag=N,a34004-Rtd=50.0,a34004-Flag=N&&5678" | nc $HOST $PORT

sleep 2

echo -e "\n3. 测试水质pH过低告警 (5.2 < 6.0阈值)"
echo "##0170QN=20250920183504003;ST=21;CN=2011;PW=123456;MN=WATER_ALARM_001;Flag=0;CP=&&DataTime=20250920183504,w01001-Rtd=5.2,w01001-Flag=N,w01003-Rtd=8.0,w01003-Flag=N&&9ABC" | nc $HOST $PORT

sleep 2

echo -e "\n4. 测试COD严重超标告警 (80.5 > 50阈值)"
echo "##0175QN=20250920183506004;ST=21;CN=2011;PW=123456;MN=WATER_ALARM_002;Flag=0;CP=&&DataTime=20250920183506,w01009-Rtd=80.5,w01009-Flag=N,w01001-Rtd=7.5,w01001-Flag=N&&DEF0" | nc $HOST $PORT

sleep 2

echo -e "\n5. 测试正常数据 (不应触发告警)"
echo "##0175QN=20250920183508005;ST=22;CN=2011;PW=123456;MN=NORMAL_TEST_001;Flag=0;CP=&&DataTime=20250920183508,a34004-Rtd=35.2,a34004-Flag=N,a21001-Rtd=0.12,a21001-Flag=N&&1122" | nc $HOST $PORT

echo -e "\n=== 测试完成 ==="
echo "请检查:"
echo "1. WebSocket客户端 (test_websocket.html) 中的告警消息"
echo "2. 服务器日志中的告警信息"
echo "3. 数据库中的告警记录:"
echo "   mysql -h127.0.0.1 -P3306 -uroot env_data_platform -e \"SELECT * FROM env_hj212_alarm_data ORDER BY received_at DESC LIMIT 10;\""