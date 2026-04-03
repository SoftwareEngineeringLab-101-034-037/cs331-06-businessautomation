package googleapi

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
)

const gmailAPIBase = "https://gmail.googleapis.com/gmail/v1/users/me"

type SendMailRequest struct {
	To       []string `json:"to"`
	CC       []string `json:"cc,omitempty"`
	BCC      []string `json:"bcc,omitempty"`
	Subject  string   `json:"subject"`
	BodyText string   `json:"body_text,omitempty"`
	BodyHTML string   `json:"body_html,omitempty"`
	ThreadID string   `json:"thread_id,omitempty"`
}

type SendMailResult struct {
	MessageID string `json:"message_id"`
	ThreadID  string `json:"thread_id,omitempty"`
}

type GmailMessage struct {
	ID           string   `json:"id"`
	ThreadID     string   `json:"thread_id"`
	Snippet      string   `json:"snippet"`
	Subject      string   `json:"subject,omitempty"`
	From         string   `json:"from,omitempty"`
	To           string   `json:"to,omitempty"`
	Date         string   `json:"date,omitempty"`
	InternalDate int64    `json:"internal_date"`
	LabelIDs     []string `json:"label_ids,omitempty"`
}

type gmailListMessagesResponse struct {
	Messages []struct {
		ID       string `json:"id"`
		ThreadID string `json:"threadId"`
	} `json:"messages"`
}

type gmailMessageMetadataResponse struct {
	ID           string   `json:"id"`
	ThreadID     string   `json:"threadId"`
	Snippet      string   `json:"snippet"`
	LabelIDs     []string `json:"labelIds"`
	InternalDate string   `json:"internalDate"`
	Payload      struct {
		Headers []struct {
			Name  string `json:"name"`
			Value string `json:"value"`
		} `json:"headers"`
	} `json:"payload"`
}

func SendEmail(client *http.Client, req SendMailRequest) (*SendMailResult, error) {
	to := compactEmails(req.To)
	cc := compactEmails(req.CC)
	bcc := compactEmails(req.BCC)

	if len(to) == 0 {
		return nil, fmt.Errorf("at least one recipient in to is required")
	}
	if strings.TrimSpace(req.Subject) == "" {
		return nil, fmt.Errorf("subject is required")
	}
	if strings.TrimSpace(req.BodyText) == "" && strings.TrimSpace(req.BodyHTML) == "" {
		return nil, fmt.Errorf("body_text or body_html is required")
	}

	rawMime := buildMimeMessage(to, cc, bcc, req.Subject, req.BodyText, req.BodyHTML)
	payload := map[string]string{"raw": base64.RawURLEncoding.EncodeToString([]byte(rawMime))}
	if strings.TrimSpace(req.ThreadID) != "" {
		payload["threadId"] = strings.TrimSpace(req.ThreadID)
	}
	body, _ := json.Marshal(payload)

	httpReq, err := http.NewRequest(http.MethodPost, gmailAPIBase+"/messages/send", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("build send request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("send gmail message: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		errBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("gmail send failed status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(errBody)))
	}

	var out struct {
		ID       string `json:"id"`
		ThreadID string `json:"threadId"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("decode gmail send response: %w", err)
	}
	return &SendMailResult{MessageID: out.ID, ThreadID: out.ThreadID}, nil
}

func ListMessages(client *http.Client, query string, afterInternalTS int64, maxResults int) ([]GmailMessage, error) {
	if maxResults <= 0 {
		maxResults = 25
	}
	if maxResults > 100 {
		maxResults = 100
	}

	q := strings.TrimSpace(query)
	if q == "" {
		q = "in:inbox"
	}
	if afterInternalTS > 0 {
		afterSeconds := afterInternalTS / 1000
		q = strings.TrimSpace(q + " after:" + strconv.FormatInt(afterSeconds, 10))
	}

	endpoint := fmt.Sprintf("%s/messages?maxResults=%d&q=%s", gmailAPIBase, maxResults, urlEncode(q))
	resp, err := client.Get(endpoint)
	if err != nil {
		return nil, fmt.Errorf("gmail list messages request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		errBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("gmail list messages failed status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(errBody)))
	}

	var listResp gmailListMessagesResponse
	if err := json.NewDecoder(resp.Body).Decode(&listResp); err != nil {
		return nil, fmt.Errorf("decode gmail list messages response: %w", err)
	}

	out := make([]GmailMessage, 0, len(listResp.Messages))
	for _, item := range listResp.Messages {
		msg, err := GetMessageMetadata(client, item.ID)
		if err != nil {
			return nil, err
		}
		if msg.InternalDate <= afterInternalTS {
			continue
		}
		out = append(out, *msg)
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].InternalDate == out[j].InternalDate {
			return out[i].ID < out[j].ID
		}
		return out[i].InternalDate < out[j].InternalDate
	})
	return out, nil
}

func GetMessageMetadata(client *http.Client, messageID string) (*GmailMessage, error) {
	endpoint := fmt.Sprintf("%s/messages/%s?format=metadata&metadataHeaders=From&metadataHeaders=To&metadataHeaders=Subject&metadataHeaders=Date", gmailAPIBase, messageID)
	resp, err := client.Get(endpoint)
	if err != nil {
		return nil, fmt.Errorf("gmail get metadata request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		errBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("gmail get metadata failed status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(errBody)))
	}

	var payload gmailMessageMetadataResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("decode gmail metadata response: %w", err)
	}

	internalTS, _ := strconv.ParseInt(strings.TrimSpace(payload.InternalDate), 10, 64)
	message := &GmailMessage{
		ID:           payload.ID,
		ThreadID:     payload.ThreadID,
		Snippet:      payload.Snippet,
		InternalDate: internalTS,
		LabelIDs:     payload.LabelIDs,
	}

	for _, header := range payload.Payload.Headers {
		switch strings.ToLower(strings.TrimSpace(header.Name)) {
		case "from":
			message.From = strings.TrimSpace(header.Value)
		case "to":
			message.To = strings.TrimSpace(header.Value)
		case "subject":
			message.Subject = strings.TrimSpace(header.Value)
		case "date":
			message.Date = strings.TrimSpace(header.Value)
		}
	}

	return message, nil
}

func buildMimeMessage(to, cc, bcc []string, subject, bodyText, bodyHTML string) string {
	headers := []string{
		"MIME-Version: 1.0",
		"To: " + strings.Join(to, ", "),
		"Subject: " + subject,
	}
	if len(cc) > 0 {
		headers = append(headers, "Cc: "+strings.Join(cc, ", "))
	}
	if len(bcc) > 0 {
		headers = append(headers, "Bcc: "+strings.Join(bcc, ", "))
	}

	textBody := strings.TrimSpace(bodyText)
	htmlBody := strings.TrimSpace(bodyHTML)
	if textBody != "" && htmlBody != "" {
		boundary := fmt.Sprintf("mixed_%d", time.Now().UnixNano())
		headers = append(headers, "Content-Type: multipart/alternative; boundary="+boundary)
		return strings.Join(headers, "\r\n") +
			"\r\n\r\n" +
			"--" + boundary + "\r\n" +
			"Content-Type: text/plain; charset=\"UTF-8\"\r\n\r\n" + textBody + "\r\n" +
			"--" + boundary + "\r\n" +
			"Content-Type: text/html; charset=\"UTF-8\"\r\n\r\n" + htmlBody + "\r\n" +
			"--" + boundary + "--"
	}

	contentType := "text/plain"
	body := textBody
	if body == "" {
		contentType = "text/html"
		body = htmlBody
	}
	headers = append(headers, "Content-Type: "+contentType+"; charset=\"UTF-8\"")
	return strings.Join(headers, "\r\n") + "\r\n\r\n" + body
}

func compactEmails(values []string) []string {
	out := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		lower := strings.ToLower(trimmed)
		if _, ok := seen[lower]; ok {
			continue
		}
		seen[lower] = struct{}{}
		out = append(out, trimmed)
	}
	return out
}

func urlEncode(input string) string {
	replacer := strings.NewReplacer(
		"%", "%25",
		" ", "%20",
		"\"", "%22",
		"#", "%23",
		"&", "%26",
		"+", "%2B",
		"?", "%3F",
	)
	return replacer.Replace(input)
}
