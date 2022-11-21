package gclient

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
)

const GITHUB_API_BASE_URL string = "https://api.github.com/repos"

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

type GClient struct {
	BaseURL string
}

func (g *GClient) GetTickets(repo string) ([]GithubTicket, error) {
	request_url, err := g.getAPIBaseURL(repo)
	if err != nil {
		return []GithubTicket{}, err
	}
	res, err := sendRequest("GET", request_url+"/issues?state=all", nil)
	if err != nil {
		return []GithubTicket{}, err
	}
	if res.StatusCode != http.StatusOK {
		return []GithubTicket{}, fmt.Errorf("request to %s returned with wrong code: %v", request_url, res.Status)
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
		return fmt.Errorf("request to %s returned with wrong code: %v", t.RepositoryURL, res.Status)
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

	request_url := fmt.Sprintf("%s/issues/%d", t.RepositoryURL, t.Number)
	res, err := sendRequest("POST", request_url, requestBody)
	if err != nil {
		return err
	}
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("request to %s returned with wrong code: %v", request_url, res.Status)
	}
	return err
}

func (g *GClient) IssueHasPR(_ GithubTicket) bool {
	return false // TODO: Implement
}

func (g *GClient) getAPIBaseURL(repo string) (string, error) {
	output, err := url.Parse(repo)
	if err != nil {
		return "", fmt.Errorf("could not recognize repository URL: %v\n", err)
	}
	// output.Path has format "/OWNER/REPOSITORY", do not add another "/" between BaseURL and output.Path
	return fmt.Sprintf("%s%s", g.BaseURL, output.Path), nil
}

func sendRequest(method, url string, data []byte) (*http.Response, error) {
	client := &http.Client{}
	req, err := http.NewRequest(method, url, bytes.NewBuffer(data))
	if err != nil {
		return nil, err
	}

	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		return nil, fmt.Errorf("could not get github token")
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Authorization", "Bearer "+token)
	res, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("issues request to %s failed: %s", url, err)
	}
	return res, nil
}
