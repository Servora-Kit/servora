package biz

import (
	"bytes"
	_ "embed"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/Servora-Kit/servora/api/gen/go/conf/v1"
)

const (
	verifyEmailTmplName   = "verify_email.html"
	resetPasswordTmplName = "reset_password.html"
)

// VerifyEmailData 供 verify_email.html 使用的数据
type VerifyEmailData struct {
	Link        string // 验证链接
	ExpiryHours string // 展示用，如 "24"
}

// ResetPasswordData 供 reset_password.html 使用的数据
type ResetPasswordData struct {
	Link        string // 重置链接
	ExpiryHours string // 展示用，如 "1"
}

var (
	defaultVerifyEmailSubject   = "Verify your email"
	defaultResetPasswordSubject = "Reset your password"

	//go:embed mail_templates/verify_email.html
	embeddedVerifyEmailHTML []byte

	//go:embed mail_templates/reset_password.html
	embeddedResetPasswordHTML []byte
)

// ttlHours 将 duration 转换为整数小时字符串，最小显示 1
func ttlHours(d time.Duration) string {
	h := int(d.Hours())
	if h < 1 {
		h = 1
	}
	return strconv.Itoa(h)
}

// RenderVerifyEmail 渲染邮箱验证邮件主题与正文。
// ttl 用于在模板中显示有效期（小时数）。
// 若 conf 中配置了 template_dir 且存在 verify_email.html 则优先使用，否则使用内嵌模板。
func RenderVerifyEmail(cfg *conf.Mail, link string, ttl time.Duration) (subject string, html []byte, err error) {
	subject = defaultVerifyEmailSubject
	data := VerifyEmailData{Link: link, ExpiryHours: ttlHours(ttl)}

	if cfg != nil && cfg.GetTemplateDir() != "" {
		path := filepath.Join(cfg.GetTemplateDir(), verifyEmailTmplName)
		if b, e := renderTemplateFile(path, data); e == nil {
			return subject, b, nil
		}
	}

	b, err := renderEmbedded(embeddedVerifyEmailHTML, data)
	return subject, b, err
}

// RenderResetPassword 渲染密码重置邮件主题与正文。
// ttl 用于在模板中显示有效期（小时数）。
// 若 conf 中配置了 template_dir 且存在 reset_password.html 则优先使用，否则使用内嵌模板。
func RenderResetPassword(cfg *conf.Mail, link string, ttl time.Duration) (subject string, html []byte, err error) {
	subject = defaultResetPasswordSubject
	data := ResetPasswordData{Link: link, ExpiryHours: ttlHours(ttl)}

	if cfg != nil && cfg.GetTemplateDir() != "" {
		path := filepath.Join(cfg.GetTemplateDir(), resetPasswordTmplName)
		if b, e := renderTemplateFile(path, data); e == nil {
			return subject, b, nil
		}
	}

	b, err := renderEmbedded(embeddedResetPasswordHTML, data)
	return subject, b, err
}

// renderEmbedded 渲染内嵌模板，出错时直接 panic（模板语法在编译期即确定，运行时不应出错）
func renderEmbedded(tmpl []byte, data any) ([]byte, error) {
	t, err := template.New("").Parse(string(tmpl))
	if err != nil {
		return nil, fmt.Errorf("parse embedded template: %w", err)
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return nil, fmt.Errorf("execute embedded template: %w", err)
	}
	return buf.Bytes(), nil
}

func renderTemplateFile(path string, data any) ([]byte, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	t, err := template.New(filepath.Base(path)).Parse(string(body))
	if err != nil {
		return nil, fmt.Errorf("parse template %s: %w", path, err)
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
