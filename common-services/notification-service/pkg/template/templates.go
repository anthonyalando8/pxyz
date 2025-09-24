// pkg/notifier/template/template.go
package template

import (
	"bytes"
	"fmt"
	"html/template"
	"strings"
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
	var basePath, bodyPath, ext string

	// normalize messageType to lowercase (matches template filenames)
	tmplName := strings.ToLower(messageType)

	// normalize data into a map
	var dataMap map[string]any
	switch v := data.(type) {
	case map[string]any:
		dataMap = v
	default:
		dataMap = map[string]any{}
		if data != nil {
			dataMap["Data"] = data
		}
	}

	switch channel {
	case "email":
		ext = "html"
		basePath = fmt.Sprintf("%s/base.%s", t.emailPath, ext)
		bodyPath = fmt.Sprintf("%s/%s.%s", t.emailPath, tmplName, ext)

		tmpl, err := template.ParseFiles(basePath, bodyPath)
		if err != nil {
			return "", fmt.Errorf("parse email templates: %w", err)
		}

		var buf bytes.Buffer
		if err := tmpl.ExecuteTemplate(&buf, "base."+ext, dataMap); err != nil {
			return "", fmt.Errorf("execute email template: %w", err)
		}
		return buf.String(), nil

	case "sms":
		ext = "txt"
		basePath = fmt.Sprintf("%s/base.%s", t.smsPath, ext)
		bodyPath = fmt.Sprintf("%s/%s.%s", t.smsPath, tmplName, ext)

		tmpl, err := texttmpl.ParseFiles(basePath, bodyPath)
		if err != nil {
			return "", fmt.Errorf("parse sms templates: %w", err)
		}

		var buf bytes.Buffer
		if err := tmpl.ExecuteTemplate(&buf, "base."+ext, dataMap); err != nil {
			return "", fmt.Errorf("execute sms template: %w", err)
		}
		return buf.String(), nil

	case "whatsapp":
		ext = "txt"
		basePath = fmt.Sprintf("%s/base.%s", t.whatsappPath, ext)
		bodyPath = fmt.Sprintf("%s/%s.%s", t.whatsappPath, tmplName, ext)

		tmpl, err := texttmpl.ParseFiles(basePath, bodyPath)
		if err != nil {
			return "", fmt.Errorf("parse whatsapp templates: %w", err)
		}

		var buf bytes.Buffer
		if err := tmpl.ExecuteTemplate(&buf, "base."+ext, dataMap); err != nil {
			return "", fmt.Errorf("execute whatsapp template: %w", err)
		}
		return buf.String(), nil

	default:
		return "", fmt.Errorf("unsupported channel: %s", channel)
	}
}

