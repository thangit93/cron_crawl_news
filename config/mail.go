package config

import (
	"github.com/jordan-wright/email"
	"net/smtp"
	"os"
)

func SendEmail(subject string, htmlContent string) error {
	e := email.NewEmail()
	e.From = os.Getenv("SMTP_FROM")
	e.To = []string{os.Getenv("EMAIL_TO")}
	e.Cc = []string{"thangtd1993@gmail.com"} // always cc to me
	e.Subject = subject
	e.HTML = []byte(htmlContent)

	smtpServer := os.Getenv("SMTP_SERVER")
	smtpPort := os.Getenv("SMTP_PORT")
	smtpUser := os.Getenv("SMTP_USER")
	smtpPass := os.Getenv("SMTP_PASS")

	auth := smtp.PlainAuth("", smtpUser, smtpPass, smtpServer)
	return e.Send(smtpServer+":"+smtpPort, auth)
}