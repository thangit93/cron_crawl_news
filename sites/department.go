package sites

import (
	"fmt"
	"log"
	"net/http"
	"github.com/PuerkitoBio/goquery"
	"sync"
	"webcrawler/config"
)

func GetDepartmentNews() {
	baseURL := "https://soxaydung.hanoi.gov.vn/"
	url := baseURL + "vi-vn/tim/ket-qua/bmjDoCDhu58geMOjIGjhu5lp"

	resp, err := http.Get(url)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		log.Fatalf("Lỗi khi phân tích HTML: %v", err)
	}

	var wg sync.WaitGroup
	sem := make(chan struct{}, 5)
	doc.Find(".col-md-10 h4 a").Each(func(i int, s *goquery.Selection) {
		title := s.Text()
		href, exists := s.Attr("href")
			if exists {
				detailURL := baseURL + href
				fmt.Printf("Link %d: %s\n", i+1, detailURL)
				if config.IsLinkSent(detailURL) {
					log.Printf("✅ Đã gửi: %s\n", detailURL)
					return
				}
				sem <- struct{}{}
				wg.Add(1)
				go func(url string) {
					defer wg.Done()
					defer func() { <-sem }() // release slot
					log.Printf("🔍 Đang crawl: %s\n", url)
					crawlDepartmentNewsDetail(detailURL, title)
				}(detailURL)
			}
	})
	wg.Wait()
}

func crawlDepartmentNewsDetail(detailURL string, emailTitle string) {
	resp, err := http.Get(detailURL)
	if err != nil {
		log.Println("Lỗi khi tải trang chi tiết:", err)
		return
	}
	defer resp.Body.Close()

	docDetail, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		log.Println("Lỗi khi phân tích HTML chi tiết:", err)
		return
	}

	contentSelection := docDetail.Find(".blog-page").First()
	if contentSelection.Length() == 0 {
		log.Println("⚠️ Không tìm thấy nội dung")
		return
	}

	contentHtml, err := goquery.OuterHtml(contentSelection)

	if err != nil {
		log.Println("Lỗi khi lấy HTML content:", err)
		return
	}

	err = config.SendEmail(emailTitle, contentHtml)
	if err != nil {
		log.Println("Lỗi khi gửi email:", err)
	}
	config.MarkLinkAsSent(detailURL)
}