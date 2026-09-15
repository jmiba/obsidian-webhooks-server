package services

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net"
	"net/mail"
	"net/smtp"
	"net/textproto"
	"strconv"
	"strings"
	"time"

	"github.com/khabaroff/obsidian-webhooks-selfhosted/src/templates"
)

// SMTPConfig configures transactional email delivery through any SMTP server.
type SMTPConfig struct {
	Host      string
	Port      int
	Username  string
	Password  string
	FromEmail string
	FromName  string
	BaseURL   string
	// TLSMode must be "starttls", "tls" (implicit TLS), or "none".
	TLSMode string
}

// EmailService handles transactional email sending via SMTP.
type EmailService struct {
	config SMTPConfig
}

// NewEmailService creates a provider-neutral SMTP email service.
func NewEmailService(config SMTPConfig) *EmailService {
	config.TLSMode = strings.ToLower(strings.TrimSpace(config.TLSMode))
	config.BaseURL = strings.TrimRight(strings.TrimSpace(config.BaseURL), "/")
	if config.TLSMode == "" {
		config.TLSMode = "starttls"
	}
	return &EmailService{config: config}
}

const defaultPublicBaseURL = "https://obsidian-webhooks.khabaroff.studio"

func (s *EmailService) publicBaseURL() string {
	if s.config.BaseURL != "" {
		return s.config.BaseURL
	}
	return defaultPublicBaseURL
}

func (s *EmailService) applyPublicURLs(config *templates.EmailConfig) {
	baseURL := s.publicBaseURL()
	config.Branding.Website = baseURL
	config.Branding.DashboardURL = baseURL + "/dashboard"
	config.Branding.DocsURL = baseURL + "/guides/"
	config.Welcome.HelpText = strings.ReplaceAll(config.Welcome.HelpText, defaultPublicBaseURL, baseURL)
	for index, step := range config.Welcome.Steps {
		config.Welcome.Steps[index] = strings.ReplaceAll(step, defaultPublicBaseURL, baseURL)
	}
}

// getDefaultEmailConfig returns default email configuration as fallback
func getDefaultEmailConfig() *templates.EmailConfig {
	return &templates.EmailConfig{
		Branding: struct {
			Name         string `yaml:"name"`
			Tagline      string `yaml:"tagline"`
			Website      string `yaml:"website"`
			DashboardURL string `yaml:"dashboard_url"`
			DocsURL      string `yaml:"docs_url"`
		}{
			Name:         "Khabaroff Studio: Obsidian Webhooks",
			Tagline:      "Webhook delivery to Obsidian",
			Website:      "https://obsidian-webhooks.khabaroff.studio",
			DashboardURL: "https://obsidian-webhooks.khabaroff.studio/dashboard",
			DocsURL:      "https://obsidian-webhooks.khabaroff.studio/guides/",
		},
		Design: struct {
			PrimaryColor  string `yaml:"primary_color"`
			PrimaryHover  string `yaml:"primary_hover"`
			TextColor     string `yaml:"text_color"`
			MutedColor    string `yaml:"muted_color"`
			Background    string `yaml:"background"`
			LightBg       string `yaml:"light_bg"`
			WarningBg     string `yaml:"warning_bg"`
			WarningBorder string `yaml:"warning_border"`
			CodeBg        string `yaml:"code_bg"`
			BorderColor   string `yaml:"border_color"`
		}{
			PrimaryColor:  "#7C3AED",
			PrimaryHover:  "#7C3AED",
			TextColor:     "#0a0a0a",
			MutedColor:    "#777777",
			Background:    "#ffffff",
			LightBg:       "#f5f5f5",
			WarningBg:     "#f5f5f5",
			WarningBorder: "#7C3AED",
			CodeBg:        "#f5f5f5",
			BorderColor:   "#e5e5e5",
		},
		Subjects: struct {
			MagicLink       string `yaml:"magic_link"`
			Welcome         string `yaml:"welcome"`
			PasswordReset   string `yaml:"password_reset"`
			AccountVerified string `yaml:"account_verified"`
		}{
			MagicLink:       "Your login link — Obsidian Webhooks",
			Welcome:         "Your API keys are ready — Obsidian Webhooks",
			PasswordReset:   "Reset your password",
			AccountVerified: "Your account is verified",
		},
	}
}

// SendMagicLinkEmail sends a magic link authentication email (always English)
func (s *EmailService) SendMagicLinkEmail(ctx context.Context, toEmail, toName, magicLink string, expiryMinutes int, language string) error {
	config, err := templates.LoadEmailConfig("en")
	if err != nil {
		config = getDefaultEmailConfig()
	}
	s.applyPublicURLs(config)

	subject := config.Subjects.MagicLink

	displayName := toName
	if displayName == "" {
		displayName = "there"
	}

	data := templates.MagicLinkData{
		Name:            displayName,
		MagicLink:       magicLink,
		ExpiryMinutes:   expiryMinutes,
		BrandName:       config.Branding.Name,
		Tagline:         config.Branding.Tagline,
		Website:         config.Branding.Website,
		Greeting:        fmt.Sprintf("Hi %s,", displayName),
		Intro:           config.MagicLink.Intro,
		ButtonText:      config.MagicLink.ButtonText,
		ExpiryWarning:   fmt.Sprintf(config.MagicLink.ExpiryWarning, expiryMinutes),
		SecurityNote:    config.MagicLink.SecurityNote,
		AlternativeText: config.MagicLink.AlternativeText,
		IgnoreText:      config.MagicLink.IgnoreText,
		PrimaryColor:    config.Design.PrimaryColor,
		PrimaryHover:    config.Design.PrimaryHover,
		TextColor:       config.Design.TextColor,
		MutedColor:      config.Design.MutedColor,
		WarningBg:       config.Design.WarningBg,
		WarningBorder:   config.Design.WarningBorder,
		CodeBg:          config.Design.CodeBg,
		BorderColor:     config.Design.BorderColor,
	}

	// Render templates
	htmlBody, err := templates.RenderMagicLinkHTML(data, language)
	if err != nil {
		// Fallback to old hardcoded template
		htmlBody = s.getMagicLinkHTMLTemplate(toName, magicLink, expiryMinutes)
	}

	textBody, err := templates.RenderMagicLinkText(data, language)
	if err != nil {
		// Fallback to old hardcoded template
		textBody = s.getMagicLinkPlainTextTemplate(toName, magicLink, expiryMinutes)
	}

	err = s.send(ctx, toEmail, subject, textBody, htmlBody)
	if err != nil {
		return fmt.Errorf("failed to send magic link email to %s: %w", toEmail, err)
	}

	return nil
}

// SendWelcomeEmail sends a welcome email to new users (always English)
func (s *EmailService) SendWelcomeEmail(ctx context.Context, toEmail, toName, language string) error {
	config, err := templates.LoadEmailConfig("en")
	if err != nil {
		config = getDefaultEmailConfig()
	}
	s.applyPublicURLs(config)

	subject := config.Subjects.Welcome

	displayName := toName
	if displayName == "" {
		displayName = "there"
	}

	data := templates.WelcomeData{
		Name:           displayName,
		BrandName:      config.Branding.Name,
		Tagline:        config.Branding.Tagline,
		Website:        config.Branding.Website,
		DashboardUrl:   config.Branding.DashboardURL,
		DocsUrl:        config.Branding.DocsURL,
		Greeting:       fmt.Sprintf("Hi %s,", displayName),
		Intro:          config.Welcome.Intro,
		NowYouCan:      config.Welcome.NowYouCan,
		ButtonText:     config.Welcome.ButtonText,
		NextStepsTitle: config.Welcome.NextStepsTitle,
		HelpText:       config.Welcome.HelpText,
		Features:       config.Welcome.Features,
		Steps:          config.Welcome.Steps,
		PrimaryColor:   config.Design.PrimaryColor,
		PrimaryHover:   config.Design.PrimaryHover,
		TextColor:      config.Design.TextColor,
		MutedColor:     config.Design.MutedColor,
		LightBg:        config.Design.LightBg,
		BorderColor:    config.Design.BorderColor,
	}

	// Render templates
	htmlBody, err := templates.RenderWelcomeHTML(data, language)
	if err != nil {
		// Fallback to old hardcoded template
		htmlBody = s.getWelcomeHTMLTemplate(toName)
	}

	textBody, err := templates.RenderWelcomeText(data, language)
	if err != nil {
		// Fallback to old hardcoded template
		textBody = s.getWelcomePlainTextTemplate(toName)
	}

	err = s.send(ctx, toEmail, subject, textBody, htmlBody)
	if err != nil {
		return fmt.Errorf("failed to send welcome email to %s: %w", toEmail, err)
	}

	return nil
}

func (s *EmailService) send(ctx context.Context, toEmail, subject, textBody, htmlBody string) error {
	message, err := s.buildMessage(toEmail, subject, textBody, htmlBody)
	if err != nil {
		return err
	}
	fromAddress, _ := mail.ParseAddress(s.config.FromEmail)
	toAddress, _ := mail.ParseAddress(toEmail)

	ctxWithTimeout, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	address := net.JoinHostPort(s.config.Host, strconv.Itoa(s.config.Port))
	dialer := &net.Dialer{Timeout: 30 * time.Second}
	conn, err := dialer.DialContext(ctxWithTimeout, "tcp", address)
	if err != nil {
		return fmt.Errorf("connect to SMTP server: %w", err)
	}

	deadline := time.Now().Add(30 * time.Second)
	if contextDeadline, ok := ctxWithTimeout.Deadline(); ok && contextDeadline.Before(deadline) {
		deadline = contextDeadline
	}
	if err := conn.SetDeadline(deadline); err != nil {
		_ = conn.Close()
		return fmt.Errorf("set SMTP connection deadline: %w", err)
	}

	tlsConfig := &tls.Config{ServerName: s.config.Host, MinVersion: tls.VersionTLS12}
	if s.config.TLSMode == "tls" {
		tlsConn := tls.Client(conn, tlsConfig)
		if err := tlsConn.HandshakeContext(ctxWithTimeout); err != nil {
			_ = conn.Close()
			return fmt.Errorf("establish implicit SMTP TLS: %w", err)
		}
		conn = tlsConn
	} else if s.config.TLSMode != "starttls" && s.config.TLSMode != "none" {
		_ = conn.Close()
		return fmt.Errorf("invalid SMTP_TLS_MODE %q (expected starttls, tls, or none)", s.config.TLSMode)
	}

	client, err := smtp.NewClient(conn, s.config.Host)
	if err != nil {
		_ = conn.Close()
		return fmt.Errorf("initialize SMTP client: %w", err)
	}
	defer client.Close()

	if s.config.TLSMode == "starttls" {
		if supported, _ := client.Extension("STARTTLS"); !supported {
			return fmt.Errorf("SMTP server does not advertise STARTTLS")
		}
		if err := client.StartTLS(tlsConfig); err != nil {
			return fmt.Errorf("establish SMTP STARTTLS: %w", err)
		}
	}

	if s.config.Username != "" {
		auth := smtp.PlainAuth("", s.config.Username, s.config.Password, s.config.Host)
		if err := client.Auth(auth); err != nil {
			return fmt.Errorf("authenticate with SMTP server: %w", err)
		}
	}
	if err := client.Mail(fromAddress.Address); err != nil {
		return fmt.Errorf("set SMTP sender: %w", err)
	}
	if err := client.Rcpt(toAddress.Address); err != nil {
		return fmt.Errorf("set SMTP recipient: %w", err)
	}

	writer, err := client.Data()
	if err != nil {
		return fmt.Errorf("start SMTP message body: %w", err)
	}
	if _, err := writer.Write(message); err != nil {
		_ = writer.Close()
		return fmt.Errorf("write SMTP message body: %w", err)
	}
	if err := writer.Close(); err != nil {
		return fmt.Errorf("finish SMTP message body: %w", err)
	}
	if err := client.Quit(); err != nil {
		return fmt.Errorf("finish SMTP session: %w", err)
	}
	return nil
}

func (s *EmailService) buildMessage(toEmail, subject, textBody, htmlBody string) ([]byte, error) {
	from, err := mail.ParseAddress(s.config.FromEmail)
	if err != nil {
		return nil, fmt.Errorf("invalid SMTP_FROM_EMAIL: %w", err)
	}
	to, err := mail.ParseAddress(toEmail)
	if err != nil {
		return nil, fmt.Errorf("invalid recipient email: %w", err)
	}
	from.Name = s.config.FromName

	var body bytes.Buffer
	multipartWriter := multipart.NewWriter(&body)
	for _, part := range []struct {
		contentType string
		body        string
	}{
		{contentType: `text/plain; charset="UTF-8"`, body: textBody},
		{contentType: `text/html; charset="UTF-8"`, body: htmlBody},
	} {
		header := make(textproto.MIMEHeader)
		header.Set("Content-Type", part.contentType)
		header.Set("Content-Transfer-Encoding", "quoted-printable")
		partWriter, err := multipartWriter.CreatePart(header)
		if err != nil {
			return nil, fmt.Errorf("create email MIME part: %w", err)
		}
		quotedWriter := quotedprintable.NewWriter(partWriter)
		if _, err := quotedWriter.Write([]byte(part.body)); err != nil {
			return nil, fmt.Errorf("encode email MIME part: %w", err)
		}
		if err := quotedWriter.Close(); err != nil {
			return nil, fmt.Errorf("finish email MIME part: %w", err)
		}
	}
	if err := multipartWriter.Close(); err != nil {
		return nil, fmt.Errorf("finish email MIME body: %w", err)
	}

	cleanSubject := strings.NewReplacer("\r", " ", "\n", " ").Replace(subject)
	var message bytes.Buffer
	fmt.Fprintf(&message, "From: %s\r\n", from.String())
	fmt.Fprintf(&message, "To: %s\r\n", to.String())
	fmt.Fprintf(&message, "Subject: %s\r\n", mime.QEncoding.Encode("UTF-8", cleanSubject))
	fmt.Fprintf(&message, "Date: %s\r\n", time.Now().Format(time.RFC1123Z))
	fmt.Fprint(&message, "MIME-Version: 1.0\r\n")
	fmt.Fprintf(&message, "Content-Type: multipart/alternative; boundary=%q\r\n\r\n", multipartWriter.Boundary())
	message.Write(body.Bytes())
	return message.Bytes(), nil
}

// getMagicLinkHTMLTemplate returns the HTML template for magic link email
func (s *EmailService) getMagicLinkHTMLTemplate(name, magicLink string, expiryMinutes int) string {
	displayName := name
	if displayName == "" {
		displayName = "there"
	}

	return fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Sign in — Obsidian Webhooks</title>
</head>
<body style="margin:0;padding:0;background:#fff;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,Helvetica,Arial,sans-serif;color:#0a0a0a;line-height:1.6;">
    <table role="presentation" width="100%%" cellpadding="0" cellspacing="0" style="max-width:560px;margin:0 auto;">
        <tr><td style="padding:40px 24px 24px;border-bottom:2px solid #e5e5e5;">
            <span style="font-size:11px;font-weight:700;letter-spacing:2px;text-transform:uppercase;">OBSIDIAN WEBHOOKS</span>
        </td></tr>
        <tr><td style="padding:32px 24px;">
            <h2 style="margin:0 0 16px;font-size:20px;font-weight:700;">Hi %s,</h2>
            <p style="margin:0 0 24px;font-size:15px;color:#444;">Click the button below to sign in to your dashboard.</p>
            <table role="presentation" cellpadding="0" cellspacing="0"><tr>
                <td style="background:#7C3AED;padding:14px 32px;">
                    <a href="%s" style="color:#fff;text-decoration:none;font-size:14px;font-weight:600;">Sign in</a>
                </td>
            </tr></table>
            <table role="presentation" width="100%%" cellpadding="0" cellspacing="0" style="margin:24px 0;"><tr>
                <td style="background:#f5f5f5;padding:14px 16px;border-left:3px solid #7C3AED;">
                    <span style="font-size:13px;font-weight:600;">This link expires in %d minutes.</span><br>
                    <span style="font-size:13px;color:#777;">Single use only. Do not share.</span>
                </td>
            </tr></table>
            <p style="margin:0 0 8px;font-size:13px;color:#777;">Or copy this link into your browser:</p>
            <p style="margin:0 0 24px;font-size:12px;font-family:monospace;color:#777;word-break:break-all;background:#f5f5f5;padding:8px 12px;">%s</p>
            <p style="margin:0;font-size:13px;color:#777;">Didn't request this? Ignore this email.</p>
        </td></tr>
        <tr><td style="padding:24px;border-top:1px solid #e5e5e5;">
            <p style="margin:0 0 4px;font-size:12px;color:#777;">Khabaroff Studio: Obsidian Webhooks — Webhook delivery to Obsidian</p>
			<a href="%s" style="font-size:12px;color:#777;">%s</a>
        </td></tr>
    </table>
</body>
</html>`, displayName, magicLink, expiryMinutes, magicLink, s.publicBaseURL(), s.publicBaseURL())
}

// getMagicLinkPlainTextTemplate returns the plain text template for magic link email
func (s *EmailService) getMagicLinkPlainTextTemplate(name, magicLink string, expiryMinutes int) string {
	displayName := name
	if displayName == "" {
		displayName = "there"
	}

	return fmt.Sprintf(`Hi %s,

Click the link below to sign in to your dashboard.

%s

This link expires in %d minutes. Single use only.

Didn't request this? Ignore this email.

—
Khabaroff Studio: Obsidian Webhooks
Webhook delivery to Obsidian
%s`, displayName, magicLink, expiryMinutes, s.publicBaseURL())
}

// getWelcomeHTMLTemplate returns the HTML template for welcome email
func (s *EmailService) getWelcomeHTMLTemplate(name string) string {
	displayName := name
	if displayName == "" {
		displayName = "there"
	}

	return fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Welcome — Obsidian Webhooks</title>
</head>
<body style="margin:0;padding:0;background:#fff;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,Helvetica,Arial,sans-serif;color:#0a0a0a;line-height:1.6;">
    <table role="presentation" width="100%%" cellpadding="0" cellspacing="0" style="max-width:560px;margin:0 auto;">
        <tr><td style="padding:40px 24px 24px;border-bottom:2px solid #e5e5e5;">
            <span style="font-size:11px;font-weight:700;letter-spacing:2px;text-transform:uppercase;">OBSIDIAN WEBHOOKS</span>
        </td></tr>
        <tr><td style="padding:32px 24px;">
            <h2 style="margin:0 0 16px;font-size:20px;font-weight:700;">Hi %s,</h2>
            <p style="margin:0 0 24px;font-size:15px;color:#444;">Your account is ready.</p>
            <p style="margin:0 0 12px;font-size:14px;font-weight:600;">What you can do:</p>
            <table role="presentation" width="100%%" cellpadding="0" cellspacing="0" style="margin-bottom:8px;"><tr>
                <td style="background:#f5f5f5;padding:14px 16px;border-left:3px solid #7C3AED;">
                    <span style="font-size:14px;font-weight:600;">Send webhooks to Obsidian</span><br>
                    <span style="font-size:13px;color:#444;">Connect n8n, Make, scripts, or any HTTP client to your vault</span>
                </td>
            </tr></table>
            <table role="presentation" width="100%%" cellpadding="0" cellspacing="0" style="margin-bottom:8px;"><tr>
                <td style="background:#f5f5f5;padding:14px 16px;border-left:3px solid #7C3AED;">
                    <span style="font-size:14px;font-weight:600;">Automatic note creation</span><br>
                    <span style="font-size:13px;color:#444;">Every webhook becomes a note in the right folder</span>
                </td>
            </tr></table>
            <table role="presentation" width="100%%" cellpadding="0" cellspacing="0" style="margin-bottom:8px;"><tr>
                <td style="background:#f5f5f5;padding:14px 16px;border-left:3px solid #7C3AED;">
                    <span style="font-size:14px;font-weight:600;">Real-time and offline delivery</span><br>
                    <span style="font-size:13px;color:#444;">Real-time when online, queued when offline (up to 30 days)</span>
                </td>
            </tr></table>
            <table role="presentation" cellpadding="0" cellspacing="0" style="margin:24px 0;"><tr>
                <td style="background:#7C3AED;padding:14px 32px;">
					<a href="%s" style="color:#fff;text-decoration:none;font-size:14px;font-weight:600;">Open Dashboard</a>
                </td>
            </tr></table>
            <p style="margin:0 0 8px;font-size:14px;font-weight:600;">Quick start:</p>
            <p style="margin:0;font-size:14px;color:#444;">1. Download the Obsidian plugin from your dashboard<br>2. Paste your client_key in plugin settings<br>3. Send a test webhook — note appears in 1-3 seconds</p>
            <p style="margin:24px 0 0;font-size:13px;color:#777;">Questions? Check the <a href="https://github.com/khabaroff-studio/obsidian-webhooks-server" style="color:#0a0a0a;font-weight:600;">docs on GitHub</a> or reply to this email.</p>
        </td></tr>
        <tr><td style="padding:24px;border-top:1px solid #e5e5e5;">
            <p style="margin:0 0 4px;font-size:12px;color:#777;">Khabaroff Studio: Obsidian Webhooks — Webhook delivery to Obsidian</p>
			<a href="%s" style="font-size:12px;color:#777;">%s</a>
        </td></tr>
    </table>
</body>
</html>`, displayName, s.publicBaseURL()+"/dashboard", s.publicBaseURL(), s.publicBaseURL())
}

// getWelcomePlainTextTemplate returns the plain text template for welcome email
func (s *EmailService) getWelcomePlainTextTemplate(name string) string {
	displayName := name
	if displayName == "" {
		displayName = "there"
	}

	return fmt.Sprintf(`Hi %s,

Your account is ready.

What you can do:

— Send webhooks to Obsidian
  Connect n8n, Make, scripts, or any HTTP client to your vault

— Automatic note creation
  Every webhook becomes a note in the right folder

— Real-time and offline delivery
  Real-time when online, queued when offline (up to 30 days)

Quick start:
1. Download the Obsidian plugin from your dashboard
2. Paste your client_key in plugin settings
3. Send a test webhook — note appears in 1-3 seconds

Open your dashboard: %s

Questions? Check the docs on GitHub or reply to this email.

—
Khabaroff Studio: Obsidian Webhooks
Webhook delivery to Obsidian
%s`, displayName, s.publicBaseURL()+"/dashboard", s.publicBaseURL())
}
