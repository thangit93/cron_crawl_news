package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
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

func main() {
	ctx := context.Background()

	// Tạo service Google Sheets và Drive bằng token OAuth
	client := getClient(ctx)
	sheetSvc, err := sheets.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		log.Fatalf("❌ Không tạo được Sheets service: %v", err)
	}

	driveSvc, err := drive.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		log.Fatalf("❌ Không tạo được Drive service: %v", err)
	}

	// Danh sách sheet
	sheetTitles := []string{"Lớp 5", "Lớp 9", "Lớp 12"}

	for _, sheetTitle := range sheetTitles {
		fmt.Println("📘 Đang xử lý sheet:", sheetTitle)
		err := processSheet(ctx, sheetSvc, driveSvc, sheetTitle)
		if err != nil {
			log.Printf("❌ Lỗi sheet %s: %v\n", sheetTitle, err)
		}
	}
}

// ====================== AUTH ======================

// getClient đọc credentials.json + token.json và tạo HTTP client
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

// getTokenFromFile đọc token từ file
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

// ====================================================

// processSheet tải dữ liệu từ Google Sheets và xử lý từng link
func processSheet(ctx context.Context, sheetSvc *sheets.Service, driveSvc *drive.Service, sheetTitle string) error {
	readRange := fmt.Sprintf("%s!A2:G", sheetTitle)
	resp, err := sheetSvc.Spreadsheets.Values.Get(spreadsheetID, readRange).Do()
	if err != nil {
		return fmt.Errorf("không đọc được sheet: %v", err)
	}
	if len(resp.Values) == 0 {
		fmt.Println("Không có dữ liệu trong sheet", sheetTitle)
		return nil
	}

	classFolderID, err := createFolderIfNotExist(ctx, driveSvc, rootFolderID, sheetTitle)
	if err != nil {
		return err
	}

	var currentSubject string

	linkRegex := regexp.MustCompile(`^https?://`)
	for rowIdx, row := range resp.Values {
		if len(row) == 0 {
			continue
		}
		if subjectName, ok := row[0].(string); ok && subjectName != "" {
			currentSubject = subjectName
		}
		if currentSubject == "" {
			continue
		}

		subjectFolderID, err := createFolderIfNotExist(ctx, driveSvc, classFolderID, currentSubject)
		if err != nil {
			log.Printf("⚠️ Không tạo được thư mục môn %s: %v", currentSubject, err)
			continue
		}

		// Xử lý 3 NXB: KNTT (B,C), CTST (D,E), CD (F,G)
		publishers := []struct {
			linkCol int
			markCol string
			name    string
		}{
			{1, fmt.Sprintf("C%d", rowIdx+2), "KNTT"},
			{3, fmt.Sprintf("E%d", rowIdx+2), "CTST"},
			{5, fmt.Sprintf("G%d", rowIdx+2), "CD"},
		}

		for _, pub := range publishers {
			if len(row) <= pub.linkCol {
				continue
			}
			link, _ := row[pub.linkCol].(string)
			if !linkRegex.MatchString(link) {
				continue
			}

			pubFolderID, err := createFolderIfNotExist(ctx, driveSvc, subjectFolderID, pub.name)
			if err != nil {
				log.Printf("⚠️ Không tạo được thư mục NXB %s: %v", pub.name, err)
				continue
			}

			fmt.Printf("%s | %s | %s: %s\n", sheetTitle, currentSubject, pub.name, link)
			fileID, err := uploadFileFromURL(ctx, driveSvc, link, pubFolderID)
			if err != nil {
				log.Printf("⚠️ Không tải được file: %v", err)
				continue
			}

			fmt.Printf("✅ Upload thành công file ID: %s\n", fileID)

			// Đánh dấu “x”
			updateRange := fmt.Sprintf("%s!%s", sheetTitle, pub.markCol)
			_, err = sheetSvc.Spreadsheets.Values.Update(spreadsheetID, updateRange, &sheets.ValueRange{
				Values: [][]interface{}{{"x"}},
			}).ValueInputOption("RAW").Do()
			if err != nil {
				log.Printf("⚠️ Không cập nhật được dấu X: %v", err)
			}
		}
	}
	return nil
}

// uploadFileFromURL tải file từ URL và upload lên Google Drive
func uploadFileFromURL(ctx context.Context, driveSvc *drive.Service, url, folderID string) (string, error) {
	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d khi tải %s", resp.StatusCode, url)
	}

	fileName := filepath.Base(strings.Split(url, "?")[0])
	if fileName == "" {
		fileName = "unknown"
	}

	driveFile := &drive.File{
		Name:    fileName,
		Parents: []string{folderID},
	}

	uploaded, err := driveSvc.Files.Create(driveFile).
		Media(resp.Body).
		SupportsAllDrives(true).
		Do()
	if err != nil {
		return "", err
	}

	return uploaded.Id, nil
}

// createFolderIfNotExist tạo folder con nếu chưa có
func createFolderIfNotExist(ctx context.Context, driveSvc *drive.Service, parentID, name string) (string, error) {
	query := fmt.Sprintf("'%s' in parents and name='%s' and mimeType='application/vnd.google-apps.folder' and trashed=false", parentID, name)
	res, err := driveSvc.Files.List().Q(query).Fields("files(id, name)").Do()
	if err != nil {
		return "", err
	}

	if len(res.Files) > 0 {
		return res.Files[0].Id, nil
	}

	folder := &drive.File{
		Name:     name,
		MimeType: "application/vnd.google-apps.folder",
		Parents:  []string{parentID},
	}

	created, err := driveSvc.Files.Create(folder).SupportsAllDrives(true).Do()
	if err != nil {
		return "", err
	}
	return created.Id, nil
}
