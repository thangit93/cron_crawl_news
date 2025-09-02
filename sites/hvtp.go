package sites

import (
	"crypto/tls"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"webcrawler/config"
	"webcrawler/helpers"

	"github.com/PuerkitoBio/goquery"
)

const MAX_DAYS = 50

func GetHvtpNews() {
	baseURL := "https://hocvientuphap.edu.vn/"
	url := baseURL + "qt/thongtintuyendung/Pages/thong-tin-tuyen-dung.aspx"
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
	doc.Find(".portlet-body .top-news").Each(func(i int, s *goquery.Selection) {
		dateStr := s.Find(".col-md-12 .ico-date").Text()
		date := strings.Trim(dateStr, "()")
		diff, err := helpers.DiffDateToday(date)
		if err != nil {
			log.Fatalln(err)
		}

		if diff <= MAX_DAYS {
			href, exists := s.Find(".title-news2").Attr("href")
			title := s.Find(".title-news2").Text()
			if exists {
				detailURL := url + href
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
					crawlTPNewsDetail(title, url, baseURL)
				}(detailURL)
			}
		}
	})
	wg.Wait()
}

func crawlTPNewsDetail(title string, detailURL string, baseURL string) {
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

	contentSelection := docDetail.Find(".content-News").First()
	if contentSelection.Length() == 0 {
		log.Println("⚠️ Không tìm thấy nội dung")
		return
	}
	contentHtml, err := goquery.OuterHtml(contentSelection)
	if err != nil {
		log.Println("Lỗi khi lấy HTML content:", err)
		return
	}
	attachmentSelection := docDetail.Find(".news-other").First()
	attachmentHtml, err := updateLinkBeforeSend(attachmentSelection, baseURL)
	if err != nil {
		log.Println("Lỗi khi lấy HTML đính kèm:", err)
		return
	}
	fullEmailHtml := contentHtml + attachmentHtml
	err = config.SendEmail(title, fullEmailHtml)
	if err != nil {
		log.Println("Lỗi khi gửi email:", err)
		return
	}
	config.MarkLinkAsSent(detailURL)
}

func updateLinkBeforeSend(attachmentSelection *goquery.Selection, baseURL string) (string, error) {
	attachmentSelection.Find("a").Each(func(i int, s *goquery.Selection) {
		href, exists := s.Attr("href")
		if exists && len(href) > 0 && href[0] == '/' {
			fullURL := baseURL + href
			s.SetAttr("href", fullURL)
		}
	})
	attachmentHtml, err := goquery.OuterHtml(attachmentSelection)
	if err != nil {
		return "", err
	}
	return attachmentHtml, nil
}
