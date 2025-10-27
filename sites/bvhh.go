package sites

import (
	"crypto/tls"
	"fmt"
	"log"
	"net/http"
	"sync"
	"webcrawler/config"

	"github.com/PuerkitoBio/goquery"
)

func GetBvhhNews() {
	baseURL := "https://vienhuyethoc.vn/"
	url := baseURL + "chuyen-muc/tin-tuc/thong-bao/"
	http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
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
	keywords := []string{"tuyển", "viên chức", "thí sinh", "ứng viên", "kỳ thi"}
	doc.Find(".title a").Each(func(i int, s *goquery.Selection) {
		title := s.Text()
		if findKeyword(title, keywords) {
			href, exists := s.Attr("href")
			if exists {
				detailURL := href
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
					crawlHhNewsDetail(detailURL, title)
				}(detailURL)
			}
		}
	})
	wg.Wait()
}

func crawlHhNewsDetail(detailURL string, title string) {
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

	contentSelection := docDetail.Find(".content-text").First()
	if contentSelection.Length() == 0 {
		log.Println("⚠️ Không tìm thấy nội dung")
		return
	}
	contentHtml, err := goquery.OuterHtml(contentSelection)
	if err != nil {
		log.Println("Lỗi khi lấy HTML content:", err)
		return
	}
	
	err = config.SendEmail(title, contentHtml)
	if err != nil {
		log.Println("Lỗi khi gửi email:", err)
		return
	}
	config.MarkLinkAsSent(detailURL)
}

