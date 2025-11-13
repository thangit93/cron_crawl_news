package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/drive/v3"
	"google.golang.org/api/option"
	"google.golang.org/api/sheets/v4"
)

const (
	spreadsheetID = "12zg3ELZoHwZE0oPC0mKbrQLtWg726UBoQFo4guXPLrQ"
)

var (
	sheetSvc *sheets.Service
)

func main() {
	ctx := context.Background()
	client := getClient(ctx)

	var err error
	sheetSvc, err = sheets.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		log.Fatalf("Không tạo được Sheets service: %v", err)
	}

	// Danh sách sheet
	sheetsList := []string{"Lớp 5", "Lớp 9", "Lớp 12"}

	for _, sh := range sheetsList {
		fmt.Println("Đang xử lý:", sh)
		// Hệ thống sẽ dừng khi gặp 1 nhóm cần tải
		if processOneGroup(ctx, sh) {
			return
		}
	}

	fmt.Println("Không còn nhóm nào cần tải.")
}

// ---------------------------------------------------------------
// XỬ LÝ 1 NHÓM DUY NHẤT (Lớp + Môn + NXB)
// ---------------------------------------------------------------
func processOneGroup(ctx context.Context, sheetName string) bool {
	readRange := fmt.Sprintf("%s!A2:G", sheetName)
	resp, err := sheetSvc.Spreadsheets.Values.Get(spreadsheetID, readRange).Do()
	if err != nil {
		log.Printf("Lỗi đọc sheet %s: %v", sheetName, err)
		return false
	}

	var currentSubject string

	// Thứ tự ưu tiên NXB
	nxbList := []struct {
		name    string
		linkIdx int
		markIdx int
	}{
		{"KNTT", 1, 2}, // B - C
		{"CTST", 3, 4}, // D - E
		{"CD", 5, 6},   // F - G
	}

	for i, row := range resp.Values {
		if len(row) == 0 {
			continue
		}

		// Cột A: chủ đề mới
		if row[0] != "" {
			currentSubject = strings.TrimSpace(fmt.Sprint(row[0]))
		}
		if currentSubject == "" {
			continue
		}

		rowNum := i + 2

		// Duyệt NXB theo đúng thứ tự B → D → F
		for _, nxb := range nxbList {
			link := getSafe(row, nxb.linkIdx)
			mark := strings.ToLower(getSafe(row, nxb.markIdx))

			if link == "" || !isValidLink(link) {
				continue
			}

			if mark == "x" {
				continue
			}

			// Gặp nhóm đầu tiên cần tải → tải 1 nhóm và dừng
			return downloadGroup(sheetName, currentSubject, nxb.name, rowNum, nxb.linkIdx, nxb.markIdx)
		}
	}

	return false
}

// an toàn lấy giá trị từ hàng
func getSafe(row []interface{}, idx int) string {
	if row == nil || idx < 0 || idx >= len(row) {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(row[idx]))
}

// Kiểm tra nhóm có link chưa tải không
func hasPending(row []interface{}, linkIdx, markIdx int) bool {
	if len(row) <= markIdx {
		return false
	}
	link := strings.TrimSpace(fmt.Sprint(row[linkIdx]))
	if link == "" || !isValidLink(link) {
		return false
	}
	status := strings.ToLower(strings.TrimSpace(fmt.Sprint(row[markIdx])))
	return status != "x"
}

// ---------------------------------------------------------------
// TẢI TOÀN BỘ LINK CỦA 1 NHÓM RỒI DỪNG CHƯƠNG TRÌNH
// ---------------------------------------------------------------
func downloadGroup(sheetName, subject, publisher string, startRow, linkIdx, markIdx int) bool {
	fmt.Printf("\n=== BẮT ĐẦU NHÓM: %s → %s → %s ===\n", sheetName, subject, publisher)

	readRange := fmt.Sprintf("%s!A2:G", sheetName)
	resp, _ := sheetSvc.Spreadsheets.Values.Get(spreadsheetID, readRange).Do()

	var wg sync.WaitGroup
	semaphore := make(chan struct{}, 5)

	for i := startRow - 2; i < len(resp.Values); i++ {
		row := resp.Values[i]
		rowNum := i + 2

		mon := getSafe(row, 0)

		if mon != "" && mon != subject && rowNum != startRow {
			break
		}

		link := getSafe(row, linkIdx)
		mark := strings.ToLower(getSafe(row, markIdx))

		if link == "" || !isValidLink(link) {
			continue
		}
		if mark == "x" {
			continue
		}

		wg.Add(1)
		semaphore <- struct{}{}

		go func(url string, r int, c int) {
			defer wg.Done()
			defer func() { <-semaphore }()

			if err := downloadFileLocal(sheetName, subject, publisher, url); err != nil {
				log.Println("Lỗi tải:", url, err)
				return
			}

			markDownloaded(sheetName, r, c)

		}(link, rowNum, markIdx)
	}

	wg.Wait()
	fmt.Println("=== HOÀN TẤT NHÓM — DỪNG CHƯƠNG TRÌNH ===")
	return true
}

// ---------------------------------------------------------------
// TẢI FILE XUỐNG LOCAL
// ---------------------------------------------------------------
func downloadFileLocal(sheetName, subject, publisher, url string) error {
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("lỗi tải file: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	rawName := filepath.Base(url)
	fileName := sanitize(rawName)

	savePath := filepath.Join("documents", sheetName, subject, publisher)
	os.MkdirAll(savePath, os.ModePerm)

	filePath := filepath.Join(savePath, fileName)
	out, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = out.ReadFrom(resp.Body)
	if err != nil {
		return err
	}

	fmt.Println("Tải thành công:", filePath)
	return nil
}

// Loại ký tự cấm trong tên file
func sanitize(name string) string {
	invalid := []string{"/", "\\", "?", "%", "*", ":", "|", "\"", "<", ">"}
	for _, c := range invalid {
		name = strings.ReplaceAll(name, c, "_")
	}
	return name
}

// ---------------------------------------------------------------
func markDownloaded(sheetName string, row int, col int) {
	colLetter := string(rune('A' + col))
	rangeStr := fmt.Sprintf("%s!%s%d", sheetName, colLetter, row)

	value := &sheets.ValueRange{
		Values: [][]interface{}{{"x"}},
	}

	_, err := sheetSvc.Spreadsheets.Values.Update(spreadsheetID, rangeStr, value).
		ValueInputOption("RAW").Do()

	if err != nil {
		log.Println("Không đánh dấu được:", rangeStr)
	} else {
		log.Println("Đã đánh dấu:", rangeStr)
	}
}

func getClient(ctx context.Context) *http.Client {
	b, _ := os.ReadFile("keys/credentials.json")
	config, _ := google.ConfigFromJSON(b, drive.DriveFileScope, sheets.SpreadsheetsScope)
	tok := getTokenFromFile("keys/token.json")
	return config.Client(ctx, tok)
}

func getTokenFromFile(file string) *oauth2.Token {
	f, _ := os.Open(file)
	defer f.Close()
	var token oauth2.Token
	json.NewDecoder(f).Decode(&token)
	return &token
}

func isValidLink(s string) bool {
	return strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://")
}
