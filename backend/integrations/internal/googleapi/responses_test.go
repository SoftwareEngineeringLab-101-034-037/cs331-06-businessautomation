package googleapi

import "testing"

func TestResponseModelShapes(t *testing.T) {
	ans := Answer{QuestionID: "q1", TextAnswers: &TextAnswers{Answers: []TextAnswer{{Value: "yes"}}}}
	resp := FormResponse{ResponseID: "r1", Answers: map[string]Answer{"q1": ans}}
	if resp.Answers["q1"].TextAnswers.Answers[0].Value != "yes" {
		t.Fatalf("unexpected response model value")
	}
}
