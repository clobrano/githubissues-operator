package controllers

import (
	"context"

	"github.com/clobrano/githubissues-operator/api/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("GithubissueController", func() {

	Context("default", func() {
		var underTest *v1alpha1.GithubIssue
		var (
			expected_url         = "clearly invalid repo URL"
			expected_title       = "Op issue title"
			expected_description = "Op issue title"
		)
		BeforeEach(func() {
			underTest = &v1alpha1.GithubIssue{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "default",
				},
				Spec: v1alpha1.GithubIssueSpec{
					Repo:        expected_url,
					Title:       expected_title,
					Description: expected_description,
				},
			}

			err := k8sClient.Create(context.Background(), underTest)
			Expect(err).NotTo(HaveOccurred())
		})

		AfterEach(func() {
			err := k8sClient.Delete(context.Background(), underTest)
			Expect(err).NotTo(HaveOccurred())
		})

		When("creating a resource", func() {
			It("it should have the expected values set", func() {
				Expect(underTest.Spec.Repo).To(Equal(expected_url))
				Expect(underTest.Spec.Title).To(Equal(expected_title))
				Expect(underTest.Spec.Description).To(Equal(expected_description))
			})
		})
	})
})
