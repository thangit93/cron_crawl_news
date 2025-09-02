package main

import (
	"log"
	"sync"
	"webcrawler/config"
	"webcrawler/sites"

	"github.com/joho/godotenv"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Fatal("Không tìm thấy file .env")
	}
	if err := config.InitDB(); err != nil {
		log.Fatalf("❌ Lỗi khởi tạo DB: %v", err)
	}
	var wg = sync.WaitGroup{}
	wg.Add(1)

	//go func() {
	//	defer wg.Done()
	//	sites.GetDocs()
	//}()
	//go func() {
	//	defer wg.Done()
	//	sites.GetNews()
	//}()
	//go func() {
	//	defer wg.Done()
	//	sites.GetDepartmentNews()
	//}()
	go func() {
		defer wg.Done()
		sites.GetHvtpNews()
	}()
	wg.Wait()
}
