package gclient

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
)

const GITHUB_API_BASE_URL string = "https://api.github.com/repos"

type GithubTicket struct {
	Number        int64  `json:"number"`
	Title         string `json:"title"`
	Body          string `json:"body"`
	State         string `json:"state"`
	RepositoryURL string `json:"repository_url"`
	HasPr         bool   `json:"has_pr"`
}

type githubIssue struct {
	Number        int64             `json:"number"`
	Title         string            `json:"title"`
	Body          string            `json:"body"`
	State         string            `json:"state"`
	RepositoryURL string            `json:"repository_url"`
	PullRequest   map[string]string `json:"pull_request"`
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
	requestUrl, err := g.getAPIBaseURL(repo)
	if err != nil {
		return []GithubTicket{}, err
	}
	requestUrl += "/issues?state=all"
	res, err := sendRequest("GET", requestUrl, nil)
	if err != nil {
		return []GithubTicket{}, err
	}
	if res.StatusCode != http.StatusOK {
		return []GithubTicket{}, fmt.Errorf("request %s returned with wrong code: %v", requestUrl, res.Status)
	}
	defer res.Body.Close()

	var allIssues []githubIssue
	err = json.NewDecoder(res.Body).Decode(&allIssues)
	if err != nil {
		return []GithubTicket{}, fmt.Errorf("can't decode body: %v", err)
	}

	ticketMap := make(map[int]GithubTicket, 1)
	var ticketsWithPR []int
	for _, i := range allIssues {
		if len(i.PullRequest) == 0 {
			newTicket := GithubTicket{
				Number:        i.Number,
				Title:         i.Title,
				Body:          i.Body,
				RepositoryURL: i.RepositoryURL,
				State:         i.State,
				HasPr:         false,
			}
			ticketMap[int(newTicket.Number)] = newTicket
		} else {
			numbers := ExtractReferencedIssue(i.Body)
			for _, n := range numbers {
				ticketsWithPR = append(ticketsWithPR, n)
			}
		}
	}
	for _, number := range ticketsWithPR {
		cpy := ticketMap[number]
		cpy.HasPr = true
		ticketMap[number] = cpy
	}

	var tickets []GithubTicket
	for _, ticket := range ticketMap {
		tickets = append(tickets, ticket)
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

	requestUrl, err := g.getAPIBaseURL(t.RepositoryURL)
	if err != nil {
		return err
	}
	requestUrl += "/issues"
	res, err := sendRequest("POST", requestUrl, requestBody)
	if err != nil {
		return err
	}
	if res.StatusCode != http.StatusCreated {
		return fmt.Errorf("request %s returned with wrong code: %v", requestUrl, res.Status)
	}
	return err
}

func (g *GClient) UpdateTicket(t GithubTicket) error {
	requestBody, err := json.Marshal(map[string]string{
		"title": t.Title,
		"body":  t.Body,
		"state": t.State,
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

func (g *GClient) IssueHasPR(t GithubTicket) bool {
	return t.HasPr
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
		return nil, fmt.Errorf("could not get github token for '%s'", url)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Authorization", "Bearer "+token)
	res, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("issues request to %s failed: %s", url, err)
	}
	return res, nil
}

func ExtractReferencedIssue(body string) []int {
	var numbers []int
	re := regexp.MustCompile("[close|closes|closed|fix|fixes|fixed|resolve|resolves|resolved]:? #([0-9]+)")

	matches := re.FindAllStringSubmatch(body, -1)
	for _, parts := range matches {
		if len(parts) >= 2 {
			for _, p := range parts[1:] {
				number, err := strconv.Atoi(p)
				if err != nil {
					continue
				}
				numbers = append(numbers, number)
			}
		}
	}
	return numbers
}
