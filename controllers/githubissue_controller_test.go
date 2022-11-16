package controllers

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/clobrano/githubissues-operator/api/v1alpha1"
	"github.com/clobrano/githubissues-operator/controllers/gclient"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
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
			secret    *corev1.Secret
			myClient  client.WithWatch
			sch       *runtime.Scheme
		)

		BeforeEach(func() {
			underTest = newGithubIssue("first issue", "issue has been assigned")
			objs := []runtime.Object{underTest}
			sch = scheme.Scheme
			sch.AddKnownTypes(v1alpha1.SchemeBuilder.GroupVersion, underTest)
			myClient = fake.NewFakeClient(objs...)
			secret = newGithubTokenSecret()
			myClient.Create(context.Background(), secret)
		})
		AfterEach(func() {
			err := myClient.Delete(context.Background(), secret)
			Expect(err).NotTo(HaveOccurred())
		})

		When("the issue does not exist", func() {
			gclient := newGithubFakeClient([]gclient.GithubTicket{})
			Expect(gclient.SpyTicket).To(BeNil())

			It("it should create it", func() {
				r := &GithubIssueReconciler{myClient, sch, &gclient}
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
				Expect(gclient.SpyTicket.Body).To(Equal("issue has been assigned"))
			})
		})

		When("the issue exists without the expected description", func() {
			gclient := newGithubFakeClient([]gclient.GithubTicket{
				{Number: 1, Title: "first issue", Body: "first issue description", State: "open"},
			})
			// The first issue in the mock-ed repository is expected to be modified
			gclient.SpyTicket = &gclient.Tickets[0]

			It("it should update the ticket description", func() {
				r := &GithubIssueReconciler{myClient, sch, &gclient}
				req := reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name:      "test",
						Namespace: "default",
					},
				}
				_, err := r.Reconcile(context.TODO(), req)
				Expect(err).NotTo(HaveOccurred())
				Expect(gclient.SpyTicket.Title).To(Equal("first issue"))
				Expect(gclient.SpyTicket.Body).To(Equal("issue has been assigned"))
			})
		})

		When("the issue is Open", func() {
			gclient := newGithubFakeClient([]gclient.GithubTicket{
				{Number: 1, Title: "first issue", Body: "first issue has a PR", State: "open"},
			})
			gclient.SpyTicket = &gclient.Tickets[0]

			It("it should set corresponding Open condition", func() {
				r := &GithubIssueReconciler{myClient, sch, &gclient}
				req := reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name:      "test",
						Namespace: "default",
					},
				}
				_, err := r.Reconcile(context.TODO(), req)
				Expect(myClient.Get(context.Background(), client.ObjectKeyFromObject(underTest), underTest)).To(Succeed())
				Expect(err).NotTo(HaveOccurred())
				Expect(underTest.Status.Conditions).To(ContainElement(
					And(
						HaveField("Type", "IsOpen"),
						HaveField("Status", metav1.ConditionTrue),
					)))
			})
		})

		When("the issue is Closed", func() {
			gclient := newGithubFakeClient([]gclient.GithubTicket{
				{Number: 1, Title: "first issue", Body: "first issue", State: "closed"},
			})
			gclient.SpyTicket = &gclient.Tickets[0]

			It("it should set corresponding Closed condition", func() {
				r := &GithubIssueReconciler{myClient, sch, &gclient}
				req := reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name:      "test",
						Namespace: "default",
					},
				}
				_, err := r.Reconcile(context.TODO(), req)
				Expect(myClient.Get(context.Background(), client.ObjectKeyFromObject(underTest), underTest)).To(Succeed())
				Expect(err).NotTo(HaveOccurred())
				Expect(underTest.Status.Conditions).To(ContainElement(
					And(
						HaveField("Type", "IsOpen"),
						HaveField("Status", metav1.ConditionFalse),
					)))
			})
		})

		When("the issue has a PR", func() {
			gclient := newGithubFakeClient([]gclient.GithubTicket{
				{Number: 1, Title: "first issue", Body: "first issue has a PR", State: "open"},
			})
			gclient.SpyTicket = &gclient.Tickets[0]

			It("it should set corresponding HasPr condition", func() {
				r := &GithubIssueReconciler{myClient, sch, &gclient}
				req := reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name:      "test",
						Namespace: "default",
					},
				}
				_, err := r.Reconcile(context.TODO(), req)
				Expect(myClient.Get(context.Background(), client.ObjectKeyFromObject(underTest), underTest)).To(Succeed())
				Expect(err).NotTo(HaveOccurred())
				Expect(underTest.Status.Conditions).To(ContainElement(
					And(
						HaveField("Type", "HasPr"),
						HaveField("Status", metav1.ConditionTrue),
					)))
			})
		})
		When("the issue has not a PR", func() {
			gclient := newGithubFakeClient([]gclient.GithubTicket{
				{Number: 1, Title: "first issue", Body: "first issue", State: "open"},
			})
			gclient.SpyTicket = &gclient.Tickets[0]

			It("it should unset corresponding HasPr condition", func() {
				r := &GithubIssueReconciler{myClient, sch, &gclient}
				req := reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name:      "test",
						Namespace: "default",
					},
				}
				_, err := r.Reconcile(context.TODO(), req)
				Expect(myClient.Get(context.Background(), client.ObjectKeyFromObject(underTest), underTest)).To(Succeed())
				Expect(err).NotTo(HaveOccurred())
				Expect(underTest.Status.Conditions).To(ContainElement(
					And(
						HaveField("Type", "HasPr"),
						HaveField("Status", metav1.ConditionFalse),
					)))
			})
		})
	})
})

func newGithubTokenSecret() *corev1.Secret {
	data := map[string][]byte{
		"GITHUB_TOKEN": []byte("gitub_fake_token"),
	}
	object := metav1.ObjectMeta{Name: "gh-token-secret", Namespace: "default"}
	secret := &corev1.Secret{Data: data, ObjectMeta: object}
	out, _ := json.Marshal(secret)
	fmt.Println(string(out))
	return secret
}

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
	Tickets   []gclient.GithubTicket
	SpyTicket *gclient.GithubTicket
}

func newGithubFakeClient(tickets []gclient.GithubTicket) GithubFakeClient {
	return GithubFakeClient{tickets, nil}
}

func (g *GithubFakeClient) GetTickets(_ string) ([]gclient.GithubTicket, error) {
	return g.Tickets, nil
}

func (g *GithubFakeClient) CreateTicket(t gclient.GithubTicket) error {
	g.SpyTicket = &t
	return nil
}

func (g *GithubFakeClient) UpdateTicket(t gclient.GithubTicket) error {
	g.SpyTicket.Title = t.Title
	g.SpyTicket.Body = t.Body
	return nil
}

func (g GithubFakeClient) IssueHasPR(t gclient.GithubTicket) bool {
	ret := strings.Contains(t.Body, "has a PR")
	return ret
}
