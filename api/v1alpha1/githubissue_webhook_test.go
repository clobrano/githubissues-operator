package v1alpha1

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Githubissues Validation", func() {
	Context("creating Githubissues CR", func() {
		When("the repository is unreachable", func() {
			It("should be rejected", func() {
				ut := newGithubIssueWithRepo("https://github.com/unreachable/repository")
				err := ut.ValidateCreate()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("Repo " + ut.Spec.Repo + " is unreachable"))
			})
		})
		When("the repository is reachable", func() {
			It("should be accepted", func() {
				ut := newGithubIssue()
				err := ut.ValidateCreate()
				Expect(err).ToNot(HaveOccurred())
			})
		})
		When("another CR with same Repo and Title exists", func() {
			It("should be rejected", func() {
				original := newGithubIssue()
				ut := newGithubIssue()
				ut.Name += "-copy"
				Expect(k8sClient.Create(context.Background(), original)).To(Succeed())
				Expect(k8sClient.Create(context.Background(), ut)).ToNot(Succeed())
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
	ut.Name = "githubissues-sample"
	ut.Namespace = "default"
	ut.Spec.Repo = "https://github.com/clobrano/githubissues-operator"
	ut.Spec.Title = "Ticket title"
	ut.Spec.Description = "Ticket description"

	return ut

}

func newGithubIssueWithRepo(repo string) *GithubIssue {
	ut := newGithubIssue()
	ut.Spec.Repo = repo
	return ut
}
