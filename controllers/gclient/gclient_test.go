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

		expected := `[
	{"number": 1, "title": "issue 1 title", "description": "issue 1 description", "state": "open"},
	{"number": 2, "title": "issue 2 title", "description": "issue 2 description", "state": "closed"}
			]`
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, expected)
		}))
		defer ts.Close()

		// Use NewServer URL as BaseURL to prevent sending request to the real Github servers
		underTest := gclient.GClient{BaseURL: ts.URL}
		tickets, err := underTest.GetTickets(ts.URL)
		Expect(err).To(BeNil())
		Expect(len(tickets)).To(Equal(2))
		Expect(tickets[0].State).To(Equal("open"))
		Expect(tickets[1].State).To(Equal("closed"))
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
			gclient.GithubTicket{0, "new issue title", "new issue description", "open", ts.URL})
		Expect(err).To(BeNil())
		Expect(newTicketReq).To(And(
			HaveField("Title", "new issue title"),
			HaveField("Body", "new issue description"),
		))
		os.Unsetenv("GITHUB_TOKEN")
	})
})
