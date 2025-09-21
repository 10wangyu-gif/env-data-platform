package main

import (
	"fmt"
	"github.com/env-data-platform/internal/config"
)

func main() {
	cfg := &config.DatabaseConfig{
		Host:      "localhost",
		Port:      3306,
		Name:      "env_data_platform",
		Username:  "root",
		Password:  "",
		Charset:   "utf8mb4",
		ParseTime: true,
		Loc:       "Asia%2FShanghai",
	}
	
	dsn := cfg.GetDSN()
	fmt.Printf("Generated DSN: %s\n", dsn)
}
