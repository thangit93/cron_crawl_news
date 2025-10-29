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

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/drive/v3"
	"google.golang.org/api/option"
	"google.golang.org/api/sheets/v4"
)

// Cấu hình
const (
	spreadsheetID = "12zg3ELZoHwZE0oPC0mKbrQLtWg726UBoQFo4guXPLrQ"
	rootFolderID  = "1vEXK_lzpWmELbpNQQKjZ6EK2O05oQMO5"
)

var (
	sheetSvc *sheets.Service
	driveSvc *drive.Service
)

func main() {
	ctx := context.Background()
	client := getClient(ctx)

	var err error
	sheetSvc, err = sheets.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		log.Fatalf("Không tạo được Sheets service: %v", err)
	}

	driveSvc, err = drive.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		log.Fatalf("Không tạo được Drive service: %v", err)
	}

	sheetsList := []string{"Lớp 5", "Lớp 9", "Lớp 12"}

	for _, sh := range sheetsList {
		fmt.Printf("\n📘 Đang xử lý sheet: %s\n", sh)
		readSheet(ctx, sh)
	}
}

// ---- Đọc và xử lý dữ liệu trong sheet ----
func readSheet(ctx context.Context, sheetName string) {
	readRange := fmt.Sprintf("%s!A2:G", sheetName)
	resp, err := sheetSvc.Spreadsheets.Values.Get(spreadsheetID, readRange).Do()
	if err != nil {
		log.Printf("❌ Lỗi sheet %s: không đọc được sheet: %v", sheetName, err)
		return
	}

	var currentSubject string
	for i, row := range resp.Values {
		if len(row) == 0 {
			continue
		}

		// Nếu cột A có tên môn mới
		if len(row) > 0 && row[0] != "" {
			currentSubject = strings.TrimSpace(fmt.Sprint(row[0]))
		}
		if currentSubject == "" {
			continue
		}

		// KNTT
		if len(row) > 2 {
			processPublisher(sheetName, currentSubject, "KNTT", row, i+2, 1, 2, "C")
		}
		// CTST
		if len(row) > 4 {
			processPublisher(sheetName, currentSubject, "CTST", row, i+2, 3, 4, "E")
		}
		// CD
		if len(row) > 6 {
			processPublisher(sheetName, currentSubject, "CD", row, i+2, 5, 6, "G")
		}
	}
}

func processPublisher(sheetName, subject, publisher string, row []interface{}, rowNum, linkIdx, markIdx int, markCol string) {
	link := strings.TrimSpace(fmt.Sprint(row[linkIdx]))
	status := strings.ToLower(strings.TrimSpace(fmt.Sprint(row[markIdx])))

	// Bỏ qua nếu không có link hợp lệ
	if link == "" || !isValidLink(link) {
		return
	}

	// Nếu đã có "x" thì bỏ qua file này
	if status == "x" {
		log.Printf("✅ Bỏ qua file đã tải: [Sheet: %s] [Môn: %s] [NXB: %s] [Dòng: %d]", sheetName, subject, publisher, rowNum)
		return
	}

	// Nếu chưa có "x" → tiến hành tải và upload
	log.Printf("⬇️  Đang tải file: [Sheet: %s] [Môn: %s] [NXB: %s] [Dòng: %d] → %s", sheetName, subject, publisher, rowNum, link)
	err := downloadAndUpload(sheetName, subject, publisher, link)
	if err != nil {
		log.Printf("⚠️  Không tải được file: [Sheet: %s] [Môn: %s] [NXB: %s] [Dòng: %d] | Lỗi: %v", sheetName, subject, publisher, rowNum, err)
	} else {
		markDownloaded(sheetName, rowNum, markCol)
		log.Printf("✅ Hoàn tất: [Sheet: %s] [Môn: %s] [NXB: %s] [Dòng: %d]", sheetName, subject, publisher, rowNum)
	}
}

// ---- Tải file và upload lên Drive ----
func downloadAndUpload(sheetName, subject, publisher, url string) error {
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("lỗi tải file: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("HTTP %d khi tải %s", resp.StatusCode, url)
	}

	fileName := filepath.Base(url)
	classFolderID := ensureFolderExists(sheetName, rootFolderID)
	subjectFolderID := ensureFolderExists(subject, classFolderID)
	pubFolderID := ensureFolderExists(publisher, subjectFolderID)

	driveFile := &drive.File{
		Name:    fileName,
		Parents: []string{pubFolderID},
	}

	_, err = driveSvc.Files.Create(driveFile).Media(resp.Body).Do()
	if err != nil {
		return fmt.Errorf("không upload nội dung: %v", err)
	}
	return nil
}

// ---- Tạo folder nếu chưa tồn tại ----
func ensureFolderExists(name, parentID string) string {
	q := fmt.Sprintf("name='%s' and mimeType='application/vnd.google-apps.folder' and '%s' in parents and trashed=false", name, parentID)
	r, err := driveSvc.Files.List().Q(q).Fields("files(id, name)").Do()
	if err == nil && len(r.Files) > 0 {
		return r.Files[0].Id
	}

	folder := &drive.File{
		Name:     name,
		MimeType: "application/vnd.google-apps.folder",
		Parents:  []string{parentID},
	}
	created, err := driveSvc.Files.Create(folder).Do()
	if err != nil {
		log.Fatalf("Không tạo được thư mục %s: %v", name, err)
	}
	return created.Id
}

// ---- Đánh dấu X sau khi tải ----
func markDownloaded(sheetName string, row int, col string) {
	writeRange := fmt.Sprintf("%s!%s%d", sheetName, col, row)
	valueRange := &sheets.ValueRange{
		Values: [][]interface{}{{"x"}},
	}
	_, err := sheetSvc.Spreadsheets.Values.Update(spreadsheetID, writeRange, valueRange).
		ValueInputOption("RAW").Do()
	if err != nil {
		log.Printf("⚠️ Không ghi được dấu x tại %s: %v", writeRange, err)
	} else {
		log.Printf("✏️ Đánh dấu x tại %s", writeRange)
	}
}

// ---- Hàm OAuth ----
func getClient(ctx context.Context) *http.Client {
	b, err := os.ReadFile("keys/credentials.json")
	if err != nil {
		log.Fatalf("Không đọc được credentials.json: %v", err)
	}

	config, err := google.ConfigFromJSON(b, drive.DriveFileScope, sheets.SpreadsheetsScope)
	if err != nil {
		log.Fatalf("Không parse được credentials.json: %v", err)
	}

	tok := getTokenFromFile("keys/token.json")
	return config.Client(ctx, tok)
}

func getTokenFromFile(file string) *oauth2.Token {
	f, err := os.Open(file)
	if err != nil {
		log.Fatalf("Không mở được %s: %v", file, err)
	}
	defer f.Close()

	var token oauth2.Token
	err = json.NewDecoder(f).Decode(&token)
	if err != nil {
		log.Fatalf("Không parse được token.json: %v", err)
	}
	return &token
}

// ---- Tiện ích ----
func isValidLink(s string) bool {
	return strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://")
}
