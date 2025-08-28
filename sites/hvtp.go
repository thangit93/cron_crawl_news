package sites

import (
	"fmt"
	"log"
	"net/http"
	"strings"
	"webcrawler/helpers"

	"github.com/PuerkitoBio/goquery"
)

func GetHvtpNews() {
	baseURL := "https://hocvientuphap.edu.vn/"
	url := baseURL + "qt/thongtintuyendung/Pages/thong-tin-tuyen-dung.aspx"

	resp, err := http.Get(url)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		log.Fatalf("Lỗi khi phân tích HTML: %v", err)
	}
	// var wg sync.WaitGroup
	// sem := make(chan struct{}, 5)
	doc.Find(".portlet-body .top-news").Each(func(i int, s *goquery.Selection) {
		dateStr := s.Find(".col-md-12 .ico-date").Text()
		date := strings.Trim(dateStr, "()")
		diff, err := helpers.DiffDateToday(date)
		if err != nil {
			log.Fatalln(err)
		}

		fmt.Println(date, diff)
	})
}
