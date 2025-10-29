package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

func main() {
	context.Background()

	// Đọc file credentials.json
	b, err := os.ReadFile("keys/credentials.json")
	if err != nil {
		log.Fatalf("Không đọc được credentials.json: %v", err)
	}

	// Tạo OAuth config
	config, err := google.ConfigFromJSON(b, "https://www.googleapis.com/auth/drive")
	if err != nil {
		log.Fatalf("Không parse được credentials.json: %v", err)
	}

	// Lấy token (qua trình duyệt)
	tok := getTokenFromWeb(config)

	// Lưu token.json để dùng sau này
	saveToken("token.json", tok)
	fmt.Println("✅ Token đã lưu thành công vào token.json")
}

// getTokenFromWeb mở trình duyệt để người dùng xác nhận quyền
func getTokenFromWeb(config *oauth2.Config) *oauth2.Token {
	authURL := config.AuthCodeURL("state-token", oauth2.AccessTypeOffline)
	fmt.Printf("🔗 Mở link sau để xác thực:\n%v\n\n", authURL)

	fmt.Print("👉 Nhập mã xác thực (authorization code) từ trình duyệt: ")
	var authCode string
	if _, err := fmt.Scan(&authCode); err != nil {
		log.Fatalf("Không đọc được mã xác thực: %v", err)
	}

	tok, err := config.Exchange(context.Background(), authCode)
	if err != nil {
		log.Fatalf("Không đổi được token: %v", err)
	}
	return tok
}

// saveToken ghi token ra file token.json
func saveToken(path string, token *oauth2.Token) {
	f, err := os.Create("keys/" + path)
	if err != nil {
		log.Fatalf("Không ghi được file token: %v", err)
	}
	defer f.Close()
	json.NewEncoder(f).Encode(token)
}
