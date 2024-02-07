package email

import (
	"bytes"
	"fmt"
	"html/template"
	"net/smtp"
	"os"
	"path/filepath"
)

var smtp_password = os.Getenv("SMTP_PASSWORD")

func SendEmailVerification(reciever string, recieverName string, verificationLink string) error {

	auth := smtp.PlainAuth("", "marogo142005@gmail.com", smtp_password, "smtp.gmail.com")

	filePrefix, _ := filepath.Abs("./modules/users/pkg/email")

	t, err := template.ParseFiles(filePrefix + "/email_verification.html")
	if err != nil {
		return err
	}

	var body bytes.Buffer

	mimeHeaders := "MIME-version: 1.0;\nContent-Type: text/html; charset=\"UTF-8\";\n\n"
	body.Write([]byte(fmt.Sprintf("Subject: Verify your email for Bazaar!\n%s\n\n", mimeHeaders)))

	t.Execute(&body, struct {
		UserName         string
		VerificationLink string
	}{
		UserName:         recieverName,
		VerificationLink: verificationLink,
	})

	err = smtp.SendMail("smtp.gmail.com:587", auth, "bazaar@bozhilov.me", []string{reciever}, body.Bytes())
	if err != nil {
		return err
	}

	return nil
}
