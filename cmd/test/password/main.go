package main

import (
	"fmt"
	"github.com/env-data-platform/internal/auth"
)

func main() {
	pm := auth.NewPasswordManager()

	// 为admin123生成正确的Argon2id密码散列
	hash, err := pm.HashPassword("admin123")
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("Admin password hash: %s\n", hash)

	// 验证密码
	valid, err := pm.VerifyPassword("admin123", hash)
	if err != nil {
		fmt.Printf("Error verifying: %v\n", err)
		return
	}

	fmt.Printf("Password valid: %v\n", valid)
}