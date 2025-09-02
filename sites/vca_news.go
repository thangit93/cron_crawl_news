package sites

import (
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"log"
	"net/http"
	"strings"
	"sync"
	"webcrawler/config"
)

func GetNews() {
	baseURL := "https://vca.org.vn/"
	url := baseURL + "tin-vca-c28.html"

	resp, err := http.Get(url)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		log.Fatalf("Lỗi khi phân tích HTML: %v", err)
	}

	keywords := []string{"kỳ thi", "tuyển dụng", "thí sinh"}

	var wg sync.WaitGroup
	sem := make(chan struct{}, 5)

	doc.Find(".title-5 a").Each(func(i int, s *goquery.Selection) {
		title := s.Text()
		if findKeyword(title, keywords) {
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
					crawlNewsDetail(detailURL, title)
				}(detailURL)
			}
		}
	})
	wg.Wait()
}

func findKeyword(s string, keywords []string) bool {
	lower := strings.ToLower(s)
	for _, kw := range keywords {
		if strings.Contains(lower, kw) {
			return true
		}
	}
	return false
}

func crawlNewsDetail(detailURL string, emailTitle string) {
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

	contentSelection := docDetail.Find(".content-items").First()
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
		return
	}
	config.MarkLinkAsSent(detailURL)
}
