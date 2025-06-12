package main

import (
	"fmt"
	"log"
	"net/http"
	"net/smtp"
	"strings"

	"github.com/jordan-wright/email"
	"github.com/PuerkitoBio/goquery"
)

func main() {
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

	doc.Find("table.table-bordered tbody tr td a").Each(func(i int, s *goquery.Selection) {
		href, exists := s.Attr("href")
		if exists {
			fmt.Printf("Link %d: %s\n", i+1, baseURL + href)
			detailURL := baseURL + href
			crawlDetail(detailURL, baseURL)
		}
	})
}

func crawlDetail(detailURL string, baseURL string) {
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
	e.From = "Tên của bạn <your.email@gmail.com>"
	e.To = []string{"nguoinhan@example.com"}
	e.Subject = subject
	e.HTML = []byte(htmlContent)

	// SMTP cấu hình (dành cho Gmail hoặc dịch vụ khác)
	smtpServer := "smtp.ethereal.email"
	smtpPort := "587"
	smtpUser := "elliot65@ethereal.email"
	smtpPass := "NGYDg7jt5PmXZHaw1w" // Dùng App Password nếu là Gmail

	auth := smtp.PlainAuth("", smtpUser, smtpPass, smtpServer)
	return e.Send(smtpServer+":"+smtpPort, auth)
}