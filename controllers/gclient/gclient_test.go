package gclient_test

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"

	"github.com/clobrano/githubissues-operator/controllers/gclient"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Github client", func() {
	It("can get a list of issues from a repository", func() {
		os.Setenv("GITHUB_TOKEN", "fake github token")

		expected := `
		[
			{
				"number": 1,
				"title": "issue 1 title",
				"body": "issue 1 description",
				"state": "open"
			},
			{
				"number": 2,
				"title": "issue 2 title",
				"body": "issue 2 description",
				"state": "closed"
			},
			{
				"number": 3,
				"title": "issue 3 title",
				"body": "issue 3 description",
				"state": "open"
			},
			{
				"number": 11,
				"title": "PR to fix issue 3",
				"body": "this is a PR and shall not count\nFixes: #3",
				"state": "open",
				"pull_request": {
					"diff_url":"just a field to have non-empty pull_request field"
				}
			},
			{
				"number": 4,
				"title": "PR",
				"body": "this is a PR and shall not count",
				"state": "open",
				"pull_request": {
					"diff_url":"just a field to have non-empty pull_request field"
				}
			}
		]`
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, expected)
		}))
		defer ts.Close()

		wanted := []gclient.GithubTicket{
			{1, "issue 1 title", "issue 1 description", "open", "", false},
			{2, "issue 2 title", "issue 2 description", "closed", "", false},
			{3, "issue 3 title", "issue 3 description", "open", "", true},
		}
		// Use NewServer URL as BaseURL to prevent sending request to the real Github servers
		underTest := gclient.GClient{BaseURL: ts.URL}
		tickets, err := underTest.GetTickets(ts.URL)
		Expect(err).To(BeNil())
		Expect(tickets).To(ContainElements(wanted))

		os.Unsetenv("GITHUB_TOKEN")
	})

	It("can create a new ticket", func() {
		os.Setenv("GITHUB_TOKEN", "fake github token")
		var newTicketReq gclient.GithubTicket

		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			body, err := ioutil.ReadAll(r.Body)
			if err != nil {
				w.WriteHeader(http.StatusBadRequest)
			}
			err = json.Unmarshal(body, &newTicketReq)
			if err != nil {
				fmt.Println(err)
				w.WriteHeader(http.StatusBadRequest)
			}
			w.WriteHeader(http.StatusCreated)
		}))
		defer ts.Close()

		// Use NewServer URL as BaseURL to prevent sending request to the real Github servers
		underTest := gclient.GClient{BaseURL: ts.URL}

		err := underTest.CreateTicket(
			gclient.GithubTicket{0, "new issue title", "new issue description", "open", ts.URL, false})
		Expect(err).To(BeNil())
		Expect(newTicketReq).To(And(
			HaveField("Title", "new issue title"),
			HaveField("Body", "new issue description"),
		))
		os.Unsetenv("GITHUB_TOKEN")
	})

	It("can extract closed issue numbers from PR body", func() {
		tt := []struct {
			body string
			want []int
		}{
			{"Fixes: #1", []int{1}},
			{"Fixes: #2", []int{2}},
			{"Closes: #22", []int{22}},
			{"Fix: #2", []int{2}},
			{"fixed: #2", []int{2}},
			{"fixed #2", []int{2}}, // the ":" is not required
			{"fixed: #2, fixed: #3", []int{2, 3}},
		}

		for _, tc := range tt {
			got := gclient.ExtractReferencedIssue(tc.body)
			Expect(got).To(Equal(tc.want))
		}
	})
})
