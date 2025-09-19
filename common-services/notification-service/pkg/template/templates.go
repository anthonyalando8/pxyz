// pkg/notifier/template/template.go
package template

import (
	"bytes"
	"fmt"
	"html/template"
	texttmpl "text/template"
)

type TemplateService struct {
	emailPath    string
	smsPath      string
	whatsappPath string
}

func NewTemplateService(emailPath, smsPath, whatsappPath string) *TemplateService {
	return &TemplateService{
		emailPath:    emailPath,
		smsPath:      smsPath,
		whatsappPath: whatsappPath,
	}
}

func (t *TemplateService) Render(channel, messageType string, data any) (string, error) {
	var tmplPath string

	switch channel {
	case "email":
		tmplPath = fmt.Sprintf("%s/%s.html", t.emailPath, messageType)
		tmpl, err := template.ParseFiles(tmplPath)
		if err != nil {
			return "", err
		}
		var buf bytes.Buffer
		if err := tmpl.Execute(&buf, data); err != nil {
			return "", err
		}
		return buf.String(), nil

	case "sms":
		tmplPath = fmt.Sprintf("%s/%s.txt", t.smsPath, messageType)
		tmpl, err := texttmpl.ParseFiles(tmplPath)
		if err != nil {
			return "", err
		}
		var buf bytes.Buffer
		if err := tmpl.Execute(&buf, data); err != nil {
			return "", err
		}
		return buf.String(), nil

	case "whatsapp":
		tmplPath = fmt.Sprintf("%s/%s.txt", t.whatsappPath, messageType)
		tmpl, err := texttmpl.ParseFiles(tmplPath)
		if err != nil {
			return "", err
		}
		var buf bytes.Buffer
		if err := tmpl.Execute(&buf, data); err != nil {
			return "", err
		}
		return buf.String(), nil

	default:
		return "", fmt.Errorf("unsupported channel: %s", channel)
	}
}
