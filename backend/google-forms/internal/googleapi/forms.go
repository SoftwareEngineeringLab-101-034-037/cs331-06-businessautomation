package googleapi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
)

const formsAPI = "https://forms.googleapis.com/v1/forms"

type Form struct {
	FormID       string     `json:"formId"`
	Info         FormInfo   `json:"info"`
	Items        []FormItem `json:"items,omitempty"`
	ResponderURI string     `json:"responderUri,omitempty"`
}

type FormInfo struct {
	Title         string `json:"title"`
	DocumentTitle string `json:"documentTitle,omitempty"`
	Description   string `json:"description,omitempty"`
}

type FormItem struct {
	ItemID       string        `json:"itemId,omitempty"`
	Title        string        `json:"title"`
	QuestionItem *QuestionItem `json:"questionItem,omitempty"`
}

type QuestionItem struct {
	Question Question `json:"question"`
}

type Question struct {
	QuestionID   string        `json:"questionId,omitempty"`
	Required     bool          `json:"required,omitempty"`
	TextQuestion *TextQuestion `json:"textQuestion,omitempty"`
}

type TextQuestion struct {
	Paragraph bool `json:"paragraph,omitempty"`
}

func CreateForm(client *http.Client, title string) (*Form, error) {
	body, _ := json.Marshal(map[string]interface{}{
		"info": FormInfo{Title: title},
	})
	resp, err := client.Post(formsAPI, "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create form: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("create form: status %d", resp.StatusCode)
	}
	var f Form
	return &f, json.NewDecoder(resp.Body).Decode(&f)
}

func GetForm(client *http.Client, formID string) (*Form, error) {
	resp, err := client.Get(fmt.Sprintf("%s/%s", formsAPI, formID))
	if err != nil {
		return nil, fmt.Errorf("get form: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("get form: status %d", resp.StatusCode)
	}
	var f Form
	return &f, json.NewDecoder(resp.Body).Decode(&f)
}

type batchUpdateReq struct {
	Requests []batchRequest `json:"requests"`
}

type batchRequest struct {
	CreateItem *createItemReq `json:"createItem,omitempty"`
}

type createItemReq struct {
	Item     FormItem `json:"item"`
	Location struct {
		Index int `json:"index"`
	} `json:"location"`
}

func AddQuestions(client *http.Client, formID string, items []FormItem) error {
	reqs := make([]batchRequest, len(items))
	for i, item := range items {
		req := createItemReq{Item: item}
		req.Location.Index = i
		reqs[i] = batchRequest{CreateItem: &req}
	}
	body, _ := json.Marshal(batchUpdateReq{Requests: reqs})
	resp, err := client.Post(
		fmt.Sprintf("%s/%s:batchUpdate", formsAPI, formID),
		"application/json",
		bytes.NewReader(body),
	)
	if err != nil {
		return fmt.Errorf("add questions: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("add questions: status %d", resp.StatusCode)
	}
	return nil
}

func SetPublished(client *http.Client, formID string, published bool) error {
	body, _ := json.Marshal(map[string]interface{}{
		"publishSettings": map[string]interface{}{
			"publishState": map[string]bool{"isPublished": published},
		},
	})
	req, err := http.NewRequest(http.MethodPost,
		fmt.Sprintf("%s/%s:setPublishSettings", formsAPI, formID),
		bytes.NewReader(body),
	)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("set published: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("set published: status %d", resp.StatusCode)
	}
	return nil
}
