package googleapi

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func jsonResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func TestFormsAPIHelpers(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodPost && req.URL.Host == "forms.googleapis.com" && req.URL.Path == "/v1/forms":
			return jsonResponse(http.StatusOK, `{"formId":"form_1","info":{"title":"Survey"},"responderUri":"https://docs.google.com/forms/d/form_1/viewform"}`), nil
		case req.Method == http.MethodGet && req.URL.Host == "forms.googleapis.com" && req.URL.Path == "/v1/forms/form_1":
			return jsonResponse(http.StatusOK, `{"formId":"form_1","info":{"title":"Survey"}}`), nil
		case req.Method == http.MethodPost && strings.HasSuffix(req.URL.Path, ":batchUpdate"):
			var body map[string]interface{}
			if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
				return nil, err
			}
			if len(body) == 0 {
				t.Fatalf("expected batch update body")
			}
			return jsonResponse(http.StatusOK, `{}`), nil
		case req.Method == http.MethodPost && strings.HasSuffix(req.URL.Path, ":setPublishSettings"):
			return jsonResponse(http.StatusOK, `{}`), nil
		case req.Method == http.MethodDelete && req.URL.Host == "www.googleapis.com" && req.URL.Path == "/drive/v3/files/form_1":
			return jsonResponse(http.StatusNoContent, ""), nil
		case req.Method == http.MethodGet && req.URL.Host == "www.googleapis.com" && req.URL.Path == "/drive/v3/files":
			if req.URL.Query().Get("pageToken") == "" {
				return jsonResponse(http.StatusOK, `{"files":[{"id":"form_1","name":"Survey","webViewLink":"https://edit/1","modifiedTime":"2026-04-06T00:00:00Z"}],"nextPageToken":"next"}`), nil
			}
			return jsonResponse(http.StatusOK, `{"files":[]}`), nil
		case req.Method == http.MethodGet && strings.HasSuffix(req.URL.Path, "/responses"):
			if req.URL.Query().Get("pageToken") == "" {
				return jsonResponse(http.StatusOK, `{"responses":[{"responseId":"resp_1","respondentEmail":"u@example.com","createTime":"2026-04-06T00:00:00Z","lastSubmittedTime":"2026-04-06T00:00:00Z","answers":{"q1":{"questionId":"q1","textAnswers":{"answers":[{"value":"yes"}]}}}}],"nextPageToken":"next"}`), nil
			}
			return jsonResponse(http.StatusOK, `{"responses":[]}`), nil
		case req.Method == http.MethodGet && strings.Contains(req.URL.Path, "/responses/resp_1"):
			return jsonResponse(http.StatusOK, `{"responseId":"resp_1","respondentEmail":"u@example.com","createTime":"2026-04-06T00:00:00Z","lastSubmittedTime":"2026-04-06T00:00:00Z","answers":{"q1":{"questionId":"q1","textAnswers":{"answers":[{"value":"yes"}]}}}}`), nil
		default:
			return jsonResponse(http.StatusNotFound, `{}`), nil
		}
	})}

	form, err := CreateForm(client, "Survey")
	if err != nil {
		t.Fatalf("CreateForm failed: %v", err)
	}
	if form.FormID != "form_1" {
		t.Fatalf("unexpected form id: %q", form.FormID)
	}

	if err := AddQuestions(client, "form_1", []FormItem{{Title: "Question 1"}}); err != nil {
		t.Fatalf("AddQuestions failed: %v", err)
	}
	if err := SetPublished(client, "form_1", true); err != nil {
		t.Fatalf("SetPublished failed: %v", err)
	}
	if err := DeleteForm(client, "form_1"); err != nil {
		t.Fatalf("DeleteForm failed: %v", err)
	}

	forms, err := ListForms(client, 1)
	if err != nil {
		t.Fatalf("ListForms failed: %v", err)
	}
	if len(forms) != 1 || forms[0].FormID != "form_1" {
		t.Fatalf("unexpected listed forms: %#v", forms)
	}

	responses, err := ListResponses(client, "form_1", "")
	if err != nil {
		t.Fatalf("ListResponses failed: %v", err)
	}
	if len(responses) != 1 || responses[0].ResponseID != "resp_1" {
		t.Fatalf("unexpected responses: %#v", responses)
	}

	resp, err := GetResponse(client, "form_1", "resp_1")
	if err != nil {
		t.Fatalf("GetResponse failed: %v", err)
	}
	if resp.ResponseID != "resp_1" || resp.Answers["q1"].TextAnswers.Answers[0].Value != "yes" {
		t.Fatalf("unexpected response payload: %#v", resp)
	}
}
