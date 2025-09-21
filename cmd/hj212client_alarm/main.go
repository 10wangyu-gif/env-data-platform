package main

import (
	"fmt"
	"net"
	"time"

	"github.com/env-data-platform/internal/hj212"
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

	parser := hj212.NewParser("HJ212-2017")

	// 测试用例 - 超过告警阈值的数据
	testCases := []struct {
		name    string
		mn      string
		st      string
		cn      string
		factors string
		desc    string
	}{
		{
			name:    "PM2.5严重超标",
			mn:      "ALARM_TEST_PM25",
			st:      "22",
			cn:      "2011",
			factors: "a34004-Rtd=200.5,a34004-Flag=N",
			desc:    "PM2.5: 200.5 μg/m³ (超过150阈值，应触发严重告警)",
		},
		{
			name:    "SO2轻微超标",
			mn:      "ALARM_TEST_SO2",
			st:      "22",
			cn:      "2011",
			factors: "a21001-Rtd=0.6,a21001-Flag=N",
			desc:    "SO2: 0.6 mg/m³ (超过0.5阈值，应触发警告告警)",
		},
		{
			name:    "水质pH过低",
			mn:      "ALARM_TEST_PH",
			st:      "21",
			cn:      "2051",
			factors: "w01001-Rtd=5.2,w01001-Flag=N",
			desc:    "pH: 5.2 (低于6.0阈值，应触发警告告警)",
		},
		{
			name:    "COD严重超标",
			mn:      "ALARM_TEST_COD",
			st:      "21",
			cn:      "2051",
			factors: "w01009-Rtd=80.5,w01009-Flag=N",
			desc:    "COD: 80.5 mg/L (超过50阈值，应触发严重告警)",
		},
		{
			name:    "正常数据",
			mn:      "NORMAL_TEST",
			st:      "22",
			cn:      "2011",
			factors: "a34004-Rtd=35.2,a34004-Flag=N,a21001-Rtd=0.12,a21001-Flag=N",
			desc:    "正常范围内的数据，不应触发告警",
		},
	}

	for i, tc := range testCases {
		fmt.Printf("\n=== 测试%d: %s ===\n", i+1, tc.name)
		fmt.Printf("描述: %s\n", tc.desc)

		// 生成QN
		qn := time.Now().Format("20060102150405") + fmt.Sprintf("%03d", i)
		dataTime := time.Now().Format("20060102150405")

		// 构建数据包
		packet := &hj212.Packet{
			QN:       qn,
			ST:       tc.st,
			CN:       tc.cn,
			PW:       "123456",
			MN:       tc.mn,
			Flag:     0,
			DataTime: time.Now(),
		}

		// 构建CP字段
		cp := fmt.Sprintf("DataTime=%s,%s", dataTime, tc.factors)
		packet.CP = cp

		// 构建完整数据包
		data, err := parser.Build(packet)
		if err != nil {
			fmt.Printf("构建数据包失败: %v\n", err)
			continue
		}
		fmt.Printf("发送数据: %s\n", string(data))

		// 发送数据
		_, err = conn.Write(data)
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
			fmt.Printf("收到响应: %s\n", string(buffer[:n]))
		}

		// 等待2秒
		time.Sleep(2 * time.Second)
	}

	fmt.Println("\n=== 告警测试完成 ===")
	fmt.Println("请检查服务器日志和WebSocket客户端中的告警信息")
}