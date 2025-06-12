package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"

	_ "github.com/go-sql-driver/mysql"
)

var DB *sql.DB

func InitDB() error {
	host := os.Getenv("DB_HOST")
	port := os.Getenv("DB_PORT")
	user := os.Getenv("DB_USER")
	pass := os.Getenv("DB_PASS")
	name := os.Getenv("DB_NAME")

	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?tls=true&charset=utf8mb4&parseTime=True",
		user, pass, host, port, name)

	var err error
	DB, err = sql.Open("mysql", dsn)
	if err != nil {
		return fmt.Errorf("lỗi kết nối DB: %w", err)
	}

	if err := DB.Ping(); err != nil {
		return fmt.Errorf("ping thất bại: %w", err)
	}

	log.Println("✅ Đã kết nối database TiDB")
	return nil
}

func IsLinkSent(url string) bool {
	var count int
	err := DB.QueryRow("SELECT COUNT(1) FROM sent_links WHERE url = ?", url).Scan(&count)
	if err != nil {
		log.Println("Lỗi kiểm tra link:", err)
		return false
	}
	return count > 0
}

func MarkLinkAsSent(url string) {
	_, err := DB.Exec("INSERT IGNORE INTO sent_links(url, sent_at) VALUES (?, NOW())", url)
	if err != nil {
		log.Println("Lỗi ghi link đã gửi:", err)
	}
}
