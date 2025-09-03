package sites

import (
	"crypto/tls"
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"log"
	"net/http"
	"sync"
	"webcrawler/config"
	"webcrawler/helpers"
)

const MAX_BVHTTDL_DAYS = 90

func GetBvhttdlNews() {
	baseURL := "https://bvhttdl.gov.vn/"
	url := baseURL + "van-ban-quan-ly.htm?keyword=tuyển+dụng&nhom=2&coquan=0&theloai=28&linhvuc=0"
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
	doc.Find(".table-data > tbody > tr").Each(func(i int, s *goquery.Selection) {
		title := s.Find("td:nth-child(2)").Text()
		href, exists := s.Find("td:nth-child(2) a").Attr("href")
		if exists {
			date := s.Find("td:nth-child(4)").Text()
			diff, err := helpers.DiffDateToday(date)
			if err != nil {
				log.Fatalln(err)
			}
			if diff <= MAX_BVHTTDL_DAYS {
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
					crawlBvhttdlNewsDetail(title, url)
				}(detailURL)
			}
		}
	})
}

func crawlBvhttdlNewsDetail(title string, detailURL string) {
	resp, err := http.Get(detailURL)
	if err != nil {
		log.Fatalln("Lỗi khi tải trang chi tiết:", err)
	}
	defer resp.Body.Close()
	newsDetail, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		log.Fatalln("Lỗi khi phân tích HTML chi tiết:", err)
	}
	contentSelection := newsDetail.Find(".table-detail").First()
	if contentSelection.Length() == 0 {
		log.Fatalln("⚠️ Không tìm thấy nội dung")
	}
	contentHtml, err := goquery.OuterHtml(contentSelection)
	if err != nil {
		log.Fatalln("Lỗi khi lấy HTML content:", err)
	}
	err = config.SendEmail(title, contentHtml)
	if err != nil {
		log.Fatalln("Lỗi khi gửi email:", err)
	}
	config.MarkLinkAsSent(detailURL)
}
