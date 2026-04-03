package googleapi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

const formsAPI = "https://forms.googleapis.com/v1/forms"
const driveFilesAPI = "https://www.googleapis.com/drive/v3/files"

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
	QuestionID         string              `json:"questionId,omitempty"`
	Required           bool                `json:"required,omitempty"`
	TextQuestion       *TextQuestion       `json:"textQuestion,omitempty"`
	ChoiceQuestion     *ChoiceQuestion     `json:"choiceQuestion,omitempty"`
	DateQuestion       *DateQuestion       `json:"dateQuestion,omitempty"`
	TimeQuestion       *TimeQuestion       `json:"timeQuestion,omitempty"`
	ScaleQuestion      *ScaleQuestion      `json:"scaleQuestion,omitempty"`
	FileUploadQuestion *FileUploadQuestion `json:"fileUploadQuestion,omitempty"`
}

type TextQuestion struct {
	Paragraph bool `json:"paragraph,omitempty"`
}

type ChoiceQuestion struct {
	Type string `json:"type,omitempty"`
}

type DateQuestion struct{}

type TimeQuestion struct{}

type ScaleQuestion struct{}

type FileUploadQuestion struct{}

type ListedForm struct {
	FormID       string `json:"form_id"`
	Title        string `json:"title"`
	ResponderURI string `json:"responder_uri,omitempty"`
	EditURI      string `json:"edit_uri,omitempty"`
	ModifiedTime string `json:"modified_time,omitempty"`
}

type driveFilesReply struct {
	Files []struct {
		ID           string `json:"id"`
		Name         string `json:"name"`
		WebViewLink  string `json:"webViewLink"`
		ModifiedTime string `json:"modifiedTime"`
	} `json:"files"`
	NextPageToken string `json:"nextPageToken,omitempty"`
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

func DeleteForm(client *http.Client, formID string) error {
	req, err := http.NewRequest(http.MethodDelete, fmt.Sprintf("%s/%s", driveFilesAPI, url.PathEscape(formID)), nil)
	if err != nil {
		return fmt.Errorf("delete form: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("delete form: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		return fmt.Errorf("delete form: status %d", resp.StatusCode)
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

// ListForms lists existing Google Forms visible to the connected account.
func ListForms(client *http.Client, pageSize int) ([]ListedForm, error) {
	if pageSize <= 0 {
		pageSize = 50
	}

	q := url.Values{}
	q.Set("q", "mimeType='application/vnd.google-apps.form' and trashed=false")
	q.Set("fields", "nextPageToken,files(id,name,webViewLink,modifiedTime)")
	q.Set("orderBy", "modifiedTime desc")
	q.Set("pageSize", fmt.Sprintf("%d", pageSize))

	forms := make([]ListedForm, 0)
	for {
		endpoint := fmt.Sprintf("%s?%s", driveFilesAPI, q.Encode())
		resp, err := client.Get(endpoint)
		if err != nil {
			return nil, fmt.Errorf("list forms: %w", err)
		}
		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			return nil, fmt.Errorf("list forms: status %d", resp.StatusCode)
		}

		var out driveFilesReply
		if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
			resp.Body.Close()
			return nil, fmt.Errorf("list forms decode: %w", err)
		}
		resp.Body.Close()

		for _, f := range out.Files {
			forms = append(forms, ListedForm{
				FormID:       f.ID,
				Title:        f.Name,
				ResponderURI: fmt.Sprintf("https://docs.google.com/forms/d/%s/viewform", f.ID),
				EditURI:      f.WebViewLink,
				ModifiedTime: f.ModifiedTime,
			})
		}

		if out.NextPageToken == "" {
			break
		}
		q.Set("pageToken", out.NextPageToken)
	}
	return forms, nil
}
