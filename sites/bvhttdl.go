package sites

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"golang.org/x/net/html"
	"log"
	"net/http"
	"regexp"
	"sync"
	"webcrawler/config"
	"webcrawler/helpers"
)

type File struct {
	FileName string `json:"FileName"`
	FileUrl  string `json:"FileUrl"`
}

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
	wg.Wait()
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
	fulContentHtmlOut, err := TransformHTML(contentHtml)
	if err != nil {
		log.Fatalln("Lỗi khi xử lý đính kèm :", err)
	}
	err = config.SendEmail(title, fulContentHtmlOut)
	if err != nil {
		log.Fatalln("Lỗi khi gửi email:", err)
	}
	config.MarkLinkAsSent(detailURL)
}

func TransformHTML(input string) (string, error) {
	doc, err := html.Parse(bytes.NewBufferString(input))
	if err != nil {
		return "", fmt.Errorf("parse HTML: %w", err)
	}

	// 1) Tìm node có id="file-placeholder"
	td := findByID(doc, "file-placeholder")
	if td == nil {
		return input, nil
	}

	// 2) Lấy toàn bộ text bên trong td (chứa <script>...)
	scriptText := nodeText(td)

	// 3) Trích JSON mảng _files bằng regex
	re := regexp.MustCompile(`(?s)_files\s*=\s*(\[[\s\S]*?\])\s*;`)
	m := re.FindStringSubmatch(scriptText)
	if len(m) < 2 {
		return input, nil // không thấy _files, giữ nguyên
	}
	jsonText := m[1]

	// 4) Parse JSON
	var files []File
	if err := json.Unmarshal([]byte(jsonText), &files); err != nil {
		return "", fmt.Errorf("parse _files JSON: %w", err)
	}

	// 5) Xóa mọi con của td và gắn lại danh sách <a> (mỗi link xuống dòng bằng <br>)
	removeAllChildren(td)
	for i, f := range files {
		td.AppendChild(makeAnchor(f.FileUrl, f.FileName))
		if i < len(files)-1 {
			td.AppendChild(&html.Node{Type: html.ElementNode, Data: "br"})
		}
	}

	// 6) Render HTML lại thành chuỗi
	var buf bytes.Buffer
	if err := html.Render(&buf, doc); err != nil {
		return "", fmt.Errorf("render HTML: %w", err)
	}
	return buf.String(), nil
}

// ===== helpers =====

func findByID(n *html.Node, id string) *html.Node {
	var q func(*html.Node) *html.Node
	q = func(cur *html.Node) *html.Node {
		if cur.Type == html.ElementNode {
			for _, a := range cur.Attr {
				if a.Key == "id" && a.Val == id {
					return cur
				}
			}
		}
		for c := cur.FirstChild; c != nil; c = c.NextSibling {
			if got := q(c); got != nil {
				return got
			}
		}
		return nil
	}
	return q(n)
}

func nodeText(n *html.Node) string {
	var buf bytes.Buffer
	var walk func(*html.Node)
	walk = func(cur *html.Node) {
		if cur.Type == html.TextNode {
			buf.WriteString(cur.Data)
		}
		for c := cur.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return buf.String()
}

func removeAllChildren(n *html.Node) {
	for c := n.FirstChild; c != nil; {
		next := c.NextSibling
		n.RemoveChild(c)
		c = next
	}
}

func makeAnchor(href, text string) *html.Node {
	a := &html.Node{Type: html.ElementNode, Data: "a", Attr: []html.Attribute{
		{Key: "href", Val: href},
		{Key: "target", Val: "_blank"},
		{Key: "rel", Val: "noopener"},
	}}
	// text node bên trong <a>
	a.AppendChild(&html.Node{Type: html.TextNode, Data: text})
	return a
}
