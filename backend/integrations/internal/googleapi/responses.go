package googleapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

type FormResponse struct {
	ResponseID        string            `json:"responseId"`
	RespondentEmail   string            `json:"respondentEmail,omitempty"`
	CreateTime        string            `json:"createTime"`
	LastSubmittedTime string            `json:"lastSubmittedTime"`
	Answers           map[string]Answer `json:"answers"`
}

type Answer struct {
	QuestionID  string       `json:"questionId"`
	TextAnswers *TextAnswers `json:"textAnswers,omitempty"`
}

type TextAnswers struct {
	Answers []TextAnswer `json:"answers"`
}

type TextAnswer struct {
	Value string `json:"value"`
}

type listResponsesReply struct {
	Responses     []FormResponse `json:"responses"`
	NextPageToken string         `json:"nextPageToken"`
}

// ListResponses fetches all responses for a form submitted after sinceTimestamp.
// sinceTimestamp should be an RFC 3339 string; pass "" to fetch all.
func ListResponses(client *http.Client, formID, sinceTimestamp string) ([]FormResponse, error) {
	var all []FormResponse
	pageToken := ""

	for {
		params := url.Values{}
		if sinceTimestamp != "" {
			params.Set("filter", fmt.Sprintf("timestamp > %s", sinceTimestamp))
		}
		if pageToken != "" {
			params.Set("pageToken", pageToken)
		}

		endpoint := fmt.Sprintf("%s/%s/responses", formsAPI, formID)
		if len(params) > 0 {
			endpoint += "?" + params.Encode()
		}

		resp, err := client.Get(endpoint)
		if err != nil {
			return nil, fmt.Errorf("list responses: %w", err)
		}

		var result listResponsesReply
		decodeErr := json.NewDecoder(resp.Body).Decode(&result)
		resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("list responses: status %d", resp.StatusCode)
		}
		if decodeErr != nil {
			return nil, fmt.Errorf("list responses decode: %w", decodeErr)
		}

		all = append(all, result.Responses...)
		if result.NextPageToken == "" {
			break
		}
		pageToken = result.NextPageToken
	}

	return all, nil
}

func GetResponse(client *http.Client, formID, responseID string) (*FormResponse, error) {
	resp, err := client.Get(fmt.Sprintf("%s/%s/responses/%s", formsAPI, formID, responseID))
	if err != nil {
		return nil, fmt.Errorf("get response: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("get response: status %d", resp.StatusCode)
	}
	var r FormResponse
	return &r, json.NewDecoder(resp.Body).Decode(&r)
}
