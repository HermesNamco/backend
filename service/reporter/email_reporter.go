package reporter

import (
	"fmt"
	"gopkg.in/gomail.v2"
	"log"

	"backend/configs"
)

// NewEmailReporter creates a new EmailReporter instance with configuration from environment
func NewEmailReporter() *EmailReporter {
	config := configs.GetConfig()
	
	return &EmailReporter{
		SMTPServer:   config.Email.SMTPServer,
		SMTPPort:     config.Email.SMTPPort,
		SMTPUsername: config.Email.SMTPUsername,
		SMTPPassword: config.Email.SMTPPassword,
		EmailFrom:    config.Email.EmailFrom,
		EmailTo:      config.Email.EmailTo,
	}
}

// EmailReporter implements the Reporter interface for sending reports via email
type EmailReporter struct {
	SMTPServer   string
	SMTPPort     int
	SMTPUsername string
	SMTPPassword string
	EmailFrom    string
	EmailTo      string
}

// For backwards compatibility
func newEmailReport() *EmailReporter {
	return NewEmailReporter()
}

// Send sends an email with the provided text content
func (e *EmailReporter) Send(text string) error {
	if e.SMTPUsername == "" || e.SMTPPassword == "" {
		return fmt.Errorf("SMTP credentials are not configured")
	}
	
	if e.EmailFrom == "" || e.EmailTo == "" {
		return fmt.Errorf("email addresses are not configured")
	}
	
	m := gomail.NewMessage()
	m.SetHeader("From", e.EmailFrom)
	m.SetHeader("To", e.EmailTo)
	m.SetHeader("Subject", "Report!")
	m.SetBody("text/html", text)

	log.Printf("Sending email report from %s to %s via %s", 
		e.EmailFrom, e.EmailTo, e.SMTPServer)

	d := gomail.NewDialer(e.SMTPServer, e.SMTPPort, e.SMTPUsername, e.SMTPPassword)

	// Send the email
	if err := d.DialAndSend(m); err != nil {
		log.Printf("Failed to send email: %v", err)
		return fmt.Errorf("send email error: %v", err)
	}
	
	log.Printf("Email report sent successfully")
	return nil
}