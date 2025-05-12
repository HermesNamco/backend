package reporter

import (
	"fmt"
	"gopkg.in/gomail.v2"
)

func newEmailReport() emailReport {
	return emailReport{}
}

type emailReport struct {
	addr     string
	userName string
	password string
}

func (e emailReport) Send(text string) error {
	m := gomail.NewMessage()
	m.SetHeader("From", e.addr)
	m.SetHeader("To", e.addr)
	m.SetHeader("Subject", "Report!")
	m.SetBody("text/html", text)
	// m.Attach("/home/Alex/lolcat.jpg")

	d := gomail.NewDialer("smtp.163.com", 25, e.userName, e.password)

	// Send the email to Bob, Cora and Dan.
	if err := d.DialAndSend(m); err != nil {
		return fmt.Errorf("send email err: %v", err)
	}
	return nil
}
