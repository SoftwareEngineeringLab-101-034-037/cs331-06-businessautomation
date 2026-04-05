package googleapi

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestGmailAPIHelpers(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodPost && strings.HasSuffix(req.URL.Path, "/messages/send"):
			var body map[string]string
			if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
				return nil, err
			}
			if body["raw"] == "" {
				t.Fatalf("expected raw payload")
			}
			return jsonResponse(http.StatusOK, `{"id":"msg_1","threadId":"thr_1"}`), nil
		case req.Method == http.MethodGet && strings.HasSuffix(req.URL.Path, "/messages"):
			return jsonResponse(http.StatusOK, `{"messages":[{"id":"msg_1","threadId":"thr_1"}]}`), nil
		case req.Method == http.MethodGet && strings.Contains(req.URL.Path, "/messages/msg_1"):
			return jsonResponse(http.StatusOK, `{"id":"msg_1","threadId":"thr_1","snippet":"hello","labelIds":["INBOX"],"internalDate":"1712361600000","payload":{"headers":[{"name":"From","value":"sender@example.com"},{"name":"To","value":"me@example.com"},{"name":"Subject","value":"Hello"},{"name":"Date","value":"Mon, 06 Apr 2026 00:00:00 +0000"}]}}`), nil
		default:
			return &http.Response{StatusCode: http.StatusNotFound, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{}`))}, nil
		}
	})}

	result, err := SendEmail(client, SendMailRequest{To: []string{"a@example.com"}, Subject: "Hi", BodyText: "hello"})
	if err != nil {
		t.Fatalf("SendEmail failed: %v", err)
	}
	if result.MessageID != "msg_1" || result.ThreadID != "thr_1" {
		t.Fatalf("unexpected send result: %#v", result)
	}

	messages, err := ListMessages(client, "in:inbox", 0, 10)
	if err != nil {
		t.Fatalf("ListMessages failed: %v", err)
	}
	if len(messages) != 1 || messages[0].ID != "msg_1" {
		t.Fatalf("unexpected messages: %#v", messages)
	}

	meta, err := GetMessageMetadata(client, "msg_1")
	if err != nil {
		t.Fatalf("GetMessageMetadata failed: %v", err)
	}
	if meta.Subject != "Hello" || meta.From != "sender@example.com" {
		t.Fatalf("unexpected metadata: %#v", meta)
	}
}
