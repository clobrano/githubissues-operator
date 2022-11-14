package controllers

import (
	"context"

	"github.com/clobrano/githubissues-operator/api/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var _ = Describe("GithubissueController", func() {

	Context("default", func() {
		var underTest *v1alpha1.GithubIssue
		var (
			expected_url         = "https://github.com/clobrano/githubissues-operator"
			expected_title       = "Op issue title"
			expected_description = "Op issue title"
		)
		BeforeEach(func() {
			underTest = newGithubIssue(expected_title, expected_description)
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

	Context("Reconciliation", func() {
		var (
			underTest *v1alpha1.GithubIssue
			client    client.WithWatch
			sch       *runtime.Scheme
		)
		BeforeEach(func() {
			underTest = newGithubIssue("first issue", "issue has been assigned")
			objs := []runtime.Object{underTest}
			sch = scheme.Scheme
			sch.AddKnownTypes(v1alpha1.SchemeBuilder.GroupVersion, underTest)
			client = fake.NewFakeClient(objs...)
		})

		When("the issue does not exist", func() {
			gclient := newGithubFakeClient([]GithubTicket{})
			Expect(gclient.SpyTicket).To(BeNil())

			It("it should create it", func() {
				r := &GithubIssueReconciler{client, sch, &gclient}
				req := reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name:      "test",
						Namespace: "default",
					},
				}
				_, err := r.Reconcile(context.TODO(), req)
				Expect(err).NotTo(HaveOccurred())
				Expect(gclient.SpyTicket).ToNot(BeNil())
				Expect(gclient.SpyTicket.Title).To(Equal("first issue"))
				Expect(gclient.SpyTicket.Description).To(Equal("issue has been assigned"))
			})
		})

		When("the issue exists without the expected description", func() {
			gclient := newGithubFakeClient([]GithubTicket{
				{"first issue", "first big issue"},
				{"second issue", "second issue description"},
			})
			// The first issue in the mock-ed repository is expected to be modified
			gclient.SpyTicket = &gclient.Tickets[0]

			It("it should update the ticket description", func() {
				r := &GithubIssueReconciler{client, sch, &gclient}
				req := reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name:      "test",
						Namespace: "default",
					},
				}
				_, err := r.Reconcile(context.TODO(), req)
				Expect(err).NotTo(HaveOccurred())
				Expect(gclient.SpyTicket.Title).To(Equal("first issue"))
				Expect(gclient.SpyTicket.Description).To(Equal("issue has been assigned"))
			})
		})

		When("the issue has a PR", func() {
			It("status should be updated", func() {

			})
		})
	})
})

func newGithubIssue(title, description string) *v1alpha1.GithubIssue {
	return &v1alpha1.GithubIssue{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "default",
		},
		Spec: v1alpha1.GithubIssueSpec{
			Repo:        "https://github.com/clobrano/githubissues-operator",
			Title:       title,
			Description: description,
		},
	}
}

type GithubFakeClient struct {
	// Tickets mocks a Github repository issue list
	Tickets   []GithubTicket
	SpyTicket *GithubTicket
}

func newGithubFakeClient(tickets []GithubTicket) GithubFakeClient {
	return GithubFakeClient{tickets, nil}
}

func (g *GithubFakeClient) GetTickets() ([]GithubTicket, error) {
	return g.Tickets, nil
}

func (g *GithubFakeClient) CreateTicket(t GithubTicket) error {
	g.SpyTicket = &t
	return nil
}

func (g *GithubFakeClient) UpdateTicket(t GithubTicket) error {
	g.SpyTicket.Title = t.Title
	g.SpyTicket.Description = t.Description
	return nil
}
