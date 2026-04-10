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
		Header:     http.Header{"Content-Type": []string{"application/json; charset=utf-8"}},
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
			if req.URL.Query().Get("pageToken") == "next" {
				return jsonResponse(http.StatusOK, `{"files":[{"id":"form_2","name":"Survey 2","webViewLink":"https://edit/2","modifiedTime":"2026-04-07T00:00:00Z"}]}`), nil
			}
			t.Fatalf("unexpected forms pageToken: %q", req.URL.Query().Get("pageToken"))
			return nil, nil
		case req.Method == http.MethodGet && strings.HasSuffix(req.URL.Path, "/responses"):
			if req.URL.Query().Get("pageToken") == "" {
				return jsonResponse(http.StatusOK, `{"responses":[{"responseId":"resp_1","respondentEmail":"u@example.com","createTime":"2026-04-06T00:00:00Z","lastSubmittedTime":"2026-04-06T00:00:00Z","answers":{"q1":{"questionId":"q1","textAnswers":{"answers":[{"value":"yes"}]}},"q_upload":{"questionId":"q_upload","fileUploadAnswers":{"answers":[{"fileId":"file_1","fileName":"resume.pdf","mimeType":"application/pdf"}]}}}}],"nextPageToken":"next"}`), nil
			}
			if req.URL.Query().Get("pageToken") == "next" {
				return jsonResponse(http.StatusOK, `{"responses":[{"responseId":"resp_2","respondentEmail":"v@example.com","createTime":"2026-04-07T00:00:00Z","lastSubmittedTime":"2026-04-07T00:00:00Z","answers":{"q1":{"questionId":"q1","textAnswers":{"answers":[{"value":"no"}]}}}}]}`), nil
			}
			t.Fatalf("unexpected responses pageToken: %q", req.URL.Query().Get("pageToken"))
			return nil, nil
		case req.Method == http.MethodGet && strings.Contains(req.URL.Path, "/responses/resp_1"):
			return jsonResponse(http.StatusOK, `{"responseId":"resp_1","respondentEmail":"u@example.com","createTime":"2026-04-06T00:00:00Z","lastSubmittedTime":"2026-04-06T00:00:00Z","answers":{"q1":{"questionId":"q1","textAnswers":{"answers":[{"value":"yes"}]}},"q_upload":{"questionId":"q_upload","fileUploadAnswers":{"answers":[{"fileId":"file_1","fileName":"resume.pdf","mimeType":"application/pdf"}]}}}}`), nil
		default:
			t.Fatalf("unexpected request: method=%s host=%s path=%s rawQuery=%s", req.Method, req.URL.Host, req.URL.Path, req.URL.RawQuery)
			return nil, nil
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
	if len(forms) != 2 || forms[0].FormID != "form_1" || forms[1].FormID != "form_2" {
		t.Fatalf("unexpected listed forms: %#v", forms)
	}

	responses, err := ListResponses(client, "form_1", "")
	if err != nil {
		t.Fatalf("ListResponses failed: %v", err)
	}
	if len(responses) != 2 || responses[0].ResponseID != "resp_1" || responses[1].ResponseID != "resp_2" {
		t.Fatalf("unexpected responses: %#v", responses)
	}
	if responses[0].RespondentEmail != "u@example.com" {
		t.Fatalf("unexpected respondent email in list response: %#v", responses[0].RespondentEmail)
	}
	listUpload, ok := responses[0].Answers["q_upload"]
	if !ok {
		t.Fatalf("expected q_upload answer in list response, got %#v", responses[0].Answers)
	}
	if listUpload.FileUploadAnswers == nil || len(listUpload.FileUploadAnswers.Answers) != 1 {
		t.Fatalf("expected file upload answers in list response, got %#v", listUpload)
	}

	resp, err := GetResponse(client, "form_1", "resp_1")
	if err != nil {
		t.Fatalf("GetResponse failed: %v", err)
	}
	if resp == nil {
		t.Fatal("expected non-nil response")
	}
	if resp.ResponseID != "resp_1" {
		t.Fatalf("unexpected response id: %#v", resp.ResponseID)
	}
	answer, ok := resp.Answers["q1"]
	if !ok {
		t.Fatalf("expected q1 answer in response, got %#v", resp.Answers)
	}
	if answer.TextAnswers == nil {
		t.Fatalf("expected text answers for q1, got %#v", answer)
	}
	if len(answer.TextAnswers.Answers) == 0 {
		t.Fatalf("expected at least one text answer for q1, got %#v", answer.TextAnswers)
	}
	if got := answer.TextAnswers.Answers[0].Value; got != "yes" {
		t.Fatalf("unexpected q1 answer value: %q", got)
	}
	if resp.RespondentEmail != "u@example.com" {
		t.Fatalf("unexpected respondent email in single response: %#v", resp.RespondentEmail)
	}
	singleUpload, ok := resp.Answers["q_upload"]
	if !ok {
		t.Fatalf("expected q_upload answer in single response, got %#v", resp.Answers)
	}
	if singleUpload.FileUploadAnswers == nil || len(singleUpload.FileUploadAnswers.Answers) != 1 {
		t.Fatalf("expected file upload answers in single response, got %#v", singleUpload)
	}
}
