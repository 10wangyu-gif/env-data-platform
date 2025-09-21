package main

import (
	"fmt"
	"net"
	"time"
)

func main() {
	fmt.Println("=== HJ212告警测试客户端 ===")

	// 连接到HJ212服务器
	conn, err := net.Dial("tcp", "localhost:8212")
	if err != nil {
		fmt.Printf("连接失败: %v\n", err)
		return
	}
	defer conn.Close()

	fmt.Println("已连接到HJ212服务器")

	// 测试数据包 - 手动构建正确的HJ212格式（参考之前成功的包格式）
	testPackets := []struct {
		name string
		data string
		desc string
	}{
		{
			name: "PM2.5严重超标",
			data: "##0192QN=20250920184000001;ST=22;CN=2011;PW=123456;MN=ALARM_TEST_001;Flag=0;CP=&&DataTime=20250920184000,a34004-Rtd=200.5,a34004-Flag=N,a21001-Rtd=0.08,a21001-Flag=N&&1234\r\n",
			desc: "PM2.5: 200.5 μg/m³ (应触发严重告警，阈值150)",
		},
		{
			name: "SO2轻微超标",
			data: "##0190QN=20250920184002002;ST=22;CN=2011;PW=123456;MN=ALARM_TEST_002;Flag=0;CP=&&DataTime=20250920184002,a21001-Rtd=0.6,a21001-Flag=N,a34004-Rtd=50.0,a34004-Flag=N&&5678\r\n",
			desc: "SO2: 0.6 mg/m³ (应触发警告告警，阈值0.5)",
		},
		{
			name: "水质pH过低",
			data: "##0188QN=20250920184004003;ST=21;CN=2051;PW=123456;MN=WATER_ALARM_001;Flag=0;CP=&&DataTime=20250920184004,w01001-Rtd=5.2,w01001-Flag=N,w01003-Rtd=8.0,w01003-Flag=N&&9ABC\r\n",
			desc: "pH: 5.2 (应触发警告告警，阈值6.0)",
		},
		{
			name: "COD严重超标",
			data: "##0192QN=20250920184006004;ST=21;CN=2051;PW=123456;MN=WATER_ALARM_002;Flag=0;CP=&&DataTime=20250920184006,w01009-Rtd=80.5,w01009-Flag=N,w01001-Rtd=7.5,w01001-Flag=N&&DEF0\r\n",
			desc: "COD: 80.5 mg/L (应触发严重告警，阈值50)",
		},
		{
			name: "正常数据",
			data: "##0190QN=20250920184008005;ST=22;CN=2011;PW=123456;MN=NORMAL_TEST_001;Flag=0;CP=&&DataTime=20250920184008,a34004-Rtd=35.2,a34004-Flag=N,a21001-Rtd=0.12,a21001-Flag=N&&1122\r\n",
			desc: "正常范围内的数据，不应触发告警",
		},
	}

	for i, packet := range testPackets {
		fmt.Printf("\n=== 测试%d: %s ===\n", i+1, packet.name)
		fmt.Printf("描述: %s\n", packet.desc)
		fmt.Printf("发送数据: %s", packet.data)

		// 发送数据
		_, err := conn.Write([]byte(packet.data))
		if err != nil {
			fmt.Printf("发送失败: %v\n", err)
			continue
		}

		// 读取响应
		buffer := make([]byte, 1024)
		conn.SetReadDeadline(time.Now().Add(3 * time.Second))
		n, err := conn.Read(buffer)
		if err != nil {
			fmt.Printf("读取响应失败: %v\n", err)
		} else {
			fmt.Printf("收到响应: %s", string(buffer[:n]))
		}

		// 等待2秒再发送下一个
		time.Sleep(2 * time.Second)
	}

	fmt.Println("\n=== 告警测试完成 ===")
	fmt.Println("请检查:")
	fmt.Println("1. 服务器日志中的告警信息")
	fmt.Println("2. WebSocket客户端中的告警消息")
	fmt.Println("3. 数据库中的告警记录")
}