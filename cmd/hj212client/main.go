package main

import (
	"fmt"
	"net"
	"time"

	"github.com/env-data-platform/internal/hj212"
)

func main() {
	// 连接到HJ212服务器
	conn, err := net.Dial("tcp", "localhost:8212")
	if err != nil {
		fmt.Printf("连接失败: %v\n", err)
		return
	}
	defer conn.Close()

	fmt.Println("已连接到HJ212服务器")

	// 创建解析器
	parser := hj212.NewParser("HJ212-2017")

	// 测试用例1: 实时数据包
	fmt.Println("\n=== 测试1: 发送实时数据包 ===")
	realtimePacket := &hj212.Packet{
		QN:   hj212.GenerateQN(),
		ST:   hj212.ST_Air,      // 空气质量监测
		CN:   hj212.CN_GetRtdData, // 实时数据
		PW:   "123456",
		MN:   "TEST001234567890",
		Flag: hj212.Flag_Online,
		CP:   "DataTime=20250920163000,a21001-Rtd=0.12,a21001-Flag=N,a21002-Rtd=0.05,a21002-Flag=N,a34004-Rtd=35.2,a34004-Flag=N",
	}

	data, err := parser.Build(realtimePacket)
	if err != nil {
		fmt.Printf("构建数据包失败: %v\n", err)
		return
	}

	fmt.Printf("发送数据: %s\n", string(data))
	_, err = conn.Write(data)
	if err != nil {
		fmt.Printf("发送失败: %v\n", err)
		return
	}

	// 读取响应
	buffer := make([]byte, 1024)
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	n, err := conn.Read(buffer)
	if err != nil {
		fmt.Printf("读取响应失败: %v\n", err)
	} else {
		fmt.Printf("收到响应: %s\n", string(buffer[:n]))
	}

	// 等待一下
	time.Sleep(1 * time.Second)

	// 测试用例2: 分钟数据包
	fmt.Println("\n=== 测试2: 发送分钟数据包 ===")
	minutePacket := &hj212.Packet{
		QN:   hj212.GenerateQN(),
		ST:   hj212.ST_Water,           // 水质监测
		CN:   hj212.CN_GetMinuteData,   // 分钟数据
		PW:   "123456",
		MN:   "WATER001234567890",
		Flag: hj212.Flag_Online,
		CP:   "DataTime=20250920163100,w01001-Rtd=7.2,w01001-Flag=N,w01003-Rtd=8.5,w01003-Flag=N,w01009-Rtd=15.3,w01009-Flag=N",
	}

	data, err = parser.Build(minutePacket)
	if err != nil {
		fmt.Printf("构建数据包失败: %v\n", err)
		return
	}

	fmt.Printf("发送数据: %s\n", string(data))
	_, err = conn.Write(data)
	if err != nil {
		fmt.Printf("发送失败: %v\n", err)
		return
	}

	// 读取响应
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	n, err = conn.Read(buffer)
	if err != nil {
		fmt.Printf("读取响应失败: %v\n", err)
	} else {
		fmt.Printf("收到响应: %s\n", string(buffer[:n]))
	}

	// 等待一下
	time.Sleep(1 * time.Second)

	// 测试用例3: 设备信息包
	fmt.Println("\n=== 测试3: 发送设备信息包 ===")
	devicePacket := &hj212.Packet{
		QN:   hj212.GenerateQN(),
		ST:   hj212.ST_System,          // 系统交互
		CN:   "3019",                   // 设备信息
		PW:   "123456",
		MN:   "DEVICE001234567890",
		Flag: hj212.Flag_Online,
		CP:   "DeviceType=Air Monitor,Version=V1.0,Location=Beijing,Manufacturer=TestCompany",
	}

	data, err = parser.Build(devicePacket)
	if err != nil {
		fmt.Printf("构建数据包失败: %v\n", err)
		return
	}

	fmt.Printf("发送数据: %s\n", string(data))
	_, err = conn.Write(data)
	if err != nil {
		fmt.Printf("发送失败: %v\n", err)
		return
	}

	// 读取响应
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	n, err = conn.Read(buffer)
	if err != nil {
		fmt.Printf("读取响应失败: %v\n", err)
	} else {
		fmt.Printf("收到响应: %s\n", string(buffer[:n]))
	}

	// 等待一下
	time.Sleep(1 * time.Second)

	// 测试用例4: 心跳包
	fmt.Println("\n=== 测试4: 发送心跳包 ===")
	heartbeatPacket := &hj212.Packet{
		QN:   hj212.GenerateQN(),
		ST:   hj212.ST_System,     // 系统交互
		CN:   hj212.CN_Response,   // 心跳包
		PW:   "123456",
		MN:   "HEARTBEAT001234567890",
		Flag: hj212.Flag_Online,
		CP:   "Status=Online",
	}

	data, err = parser.Build(heartbeatPacket)
	if err != nil {
		fmt.Printf("构建数据包失败: %v\n", err)
		return
	}

	fmt.Printf("发送数据: %s\n", string(data))
	_, err = conn.Write(data)
	if err != nil {
		fmt.Printf("发送失败: %v\n", err)
		return
	}

	// 读取响应
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	n, err = conn.Read(buffer)
	if err != nil {
		fmt.Printf("读取响应失败: %v\n", err)
	} else {
		fmt.Printf("收到响应: %s\n", string(buffer[:n]))
	}

	fmt.Println("\n=== 测试完成 ===")
}