package main

import (
	"log"
	"sync"
	"webcrawler/config"
	"webcrawler/sites"
)

func main() {
	if err := config.InitDB(); err != nil {
		log.Fatalf("❌ Lỗi khởi tạo DB: %v", err)
	}
	var wg = sync.WaitGroup{}
	wg.Add(3)

	go func() {
		defer wg.Done()
		sites.GetDocs()
	}()
	go func() {
		defer wg.Done()
		sites.GetNews()
	}()
	go func() {
		defer wg.Done()
		sites.GetDepartmentNews()
	}()
	wg.Wait()
}
