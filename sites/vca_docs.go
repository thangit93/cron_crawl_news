package sites

import (
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"github.com/jordan-wright/email"
	"log"
	"net/http"
	"net/smtp"
	"os"
	"strings"
	"sync"

	"webcrawler/config"
)

func GetDocs() {
	baseURL := "https://vca.org.vn/"
	url := baseURL + "frontend/home/search?s=Th%C3%B4ng+b%C3%A1o+tuy%E1%BB%83n+d%E1%BB%A5ng&loaivanban=&issuing_agency=&year=&submit=T%C3%ACm+ki%E1%BA%BFm"

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

	doc.Find("table.table-bordered tbody tr td a").Each(func(i int, s *goquery.Selection) {
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
				crawlDocDetail(url, baseURL)
			}(detailURL)
		}
	})
	wg.Wait()
}

func crawlDocDetail(detailURL string, baseURL string) {
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

	tableSelection := docDetail.Find("table.table.table-bordered").First()
	if tableSelection.Length() == 0 {
		log.Println("⚠️ Không tìm thấy bảng")
		return
	}

	tableHTML, emailTitle, err := updateTableBeforeSendEmail(tableSelection, baseURL)

	if err != nil {
		log.Println("Lỗi khi lấy HTML bảng:", err)
		return
	}

	if tableHTML == "" {
		log.Println("Không tìm thấy bảng để gửi email")
		return
	}

	err = sendEmail(emailTitle, tableHTML)
	if err != nil {
		log.Println("Lỗi khi gửi email:", err)
	}
	config.MarkLinkAsSent(detailURL)
}

func updateTableBeforeSendEmail(tableSelection *goquery.Selection, baseURL string) (string, string, error) {
	var emailTitle string
	tableSelection.Find("a").Each(func(i int, s *goquery.Selection) {
		href, exists := s.Attr("href")
		if exists && len(href) > 0 && href[0] == '/' {
			href = strings.Replace(href, "/upload/upload/", "upload/", 1)
			fullURL := baseURL + href
			s.SetAttr("href", fullURL)
			emailTitle = s.Text()
		}
	})

	tableHTML, err := goquery.OuterHtml(tableSelection)
	if err != nil {
		log.Println("Lỗi lấy HTML bảng:", err)
		return "", "", err
	}
	return tableHTML, emailTitle, nil
}

func sendEmail(subject string, htmlContent string) error {
	e := email.NewEmail()
	e.From = os.Getenv("SMTP_FROM")
	e.To = []string{os.Getenv("EMAIL_TO")}
	e.Subject = subject
	e.HTML = []byte(htmlContent)

	smtpServer := os.Getenv("SMTP_SERVER")
	smtpPort := os.Getenv("SMTP_PORT")
	smtpUser := os.Getenv("SMTP_USER")
	smtpPass := os.Getenv("SMTP_PASS")

	auth := smtp.PlainAuth("", smtpUser, smtpPass, smtpServer)
	return e.Send(smtpServer+":"+smtpPort, auth)
}
