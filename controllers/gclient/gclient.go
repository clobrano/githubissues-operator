package gclient

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
)

type GithubTicket struct {
	Number        int64  `json:"number"`
	Title         string `json:"title"`
	Body          string `json:"body"`
	State         string `json:"state"`
	RepositoryURL string `json:"repository_url"`
}

type GithubClient interface {
	GetTickets(string) ([]GithubTicket, error)
	CreateTicket(GithubTicket) error
	UpdateTicket(GithubTicket) error
	IssueHasPR(GithubTicket) bool
}

type GClient struct{}

func (g *GClient) GetTickets(url string) ([]GithubTicket, error) {
	res, err := sendRequest("GET", url+"/issues", nil)
	if err != nil {
		return []GithubTicket{}, err
	}
	if res.StatusCode != http.StatusOK {
		return []GithubTicket{}, fmt.Errorf("request returned with wrong code: %v", res.Status)
	}
	defer res.Body.Close()

	var tickets []GithubTicket
	err = json.NewDecoder(res.Body).Decode(&tickets)
	if err != nil {
		return []GithubTicket{}, fmt.Errorf("can't decode body: %v", err)
	}
	return tickets, nil
}

func (g *GClient) CreateTicket(t GithubTicket) error {
	requestBody, err := json.Marshal(map[string]string{
		"title": t.Title,
		"body":  t.Body,
	})
	if err != nil {
		return err
	}

	res, err := sendRequest("POST", t.RepositoryURL+"/issues", requestBody)
	if err != nil {
		return err
	}
	if res.StatusCode != http.StatusCreated {
		return fmt.Errorf("request returned with wrong code: %v", res.Status)
	}
	return err
}

func (g *GClient) UpdateTicket(t GithubTicket) error {
	requestBody, err := json.Marshal(map[string]string{
		"title": t.Title,
		"body":  t.Body,
	})
	if err != nil {
		return err
	}

	url := fmt.Sprintf("%s/issues/%d", t.RepositoryURL, t.Number)
	res, err := sendRequest("POST", url, requestBody)
	if err != nil {
		return err
	}
	if res.StatusCode != http.StatusCreated {
		return fmt.Errorf("request returned with wrong code: %v", res.Status)
	}
	return err
}

func (g *GClient) IssueHasPR(_ GithubTicket) bool {
	return false // TODO: Implement
}

func sendRequest(method, url string, data []byte) (*http.Response, error) {
	client := &http.Client{}
	req, err := http.NewRequest(method, url, bytes.NewBuffer(data))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Authorization", "Bearer "+os.Getenv("GITHUB_TOKEN"))
	res, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("issues request failed: %s", err)
	}
	return res, nil
}
