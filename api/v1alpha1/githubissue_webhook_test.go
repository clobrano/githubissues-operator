package v1alpha1

import (
	"net/http"
	"net/http/httptest"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Githubissues Validation", func() {
	Context("creating Githubissues CR", func() {
		When("the repository is unreachable", func() {
			It("should be rejected", func() {
				unreachableSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusNotFound)
				}))
				defer unreachableSrv.Close()

				ut := newGithubIssueWithRepo(unreachableSrv.URL)
				err := ut.ValidateCreate()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("Repo " + ut.Spec.Repo + " is unreachable"))
			})
		})
		When("the repository is reachable", func() {
			It("should be rejected", func() {
				reachableSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusOK)
				}))
				defer reachableSrv.Close()

				ut := newGithubIssueWithRepo(reachableSrv.URL)
				err := ut.ValidateCreate()
				Expect(err).ToNot(HaveOccurred())
			})
		})
	})

	Context("update Githubissues CR", func() {
		When("update immutable fields", func() {
			It("should be rejected", func() {
				ut := newGithubIssue()

				utCopy := ut.DeepCopy()
				utCopy.Spec.Repo = "Changed repo string"
				err := ut.ValidateUpdate(utCopy)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("could not update: Repo field is immutable"))

				utCopy = ut.DeepCopy()
				utCopy.Spec.Title = "Changed title"
				err = ut.ValidateUpdate(utCopy)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("could not update: Title field is immutable"))

				utCopy = ut.DeepCopy()
				utCopy.Spec.Repo = "Changed repo string"
				utCopy.Spec.Title = "Changed title"
				err = ut.ValidateUpdate(utCopy)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal(
					"could not update: Repo field is immutable" +
						"\n" +
						"could not update: Title field is immutable"))
			})
		})
	})
})

func newGithubIssue() *GithubIssue {
	ut := &GithubIssue{}
	ut.Name = "test"
	ut.Namespace = "default"
	ut.Spec.Repo = "https://github.com/fake/repository"
	ut.Spec.Title = "Ticket title"
	ut.Spec.Description = "Ticket description"

	return ut

}

func newGithubIssueWithRepo(repo string) *GithubIssue {
	ut := newGithubIssue()
	ut.Spec.Repo = repo
	return ut
}
