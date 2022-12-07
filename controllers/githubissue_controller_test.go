package controllers

import (
	"context"
	"fmt"

	"github.com/clobrano/githubissues-operator/api/v1alpha1"
	"github.com/clobrano/githubissues-operator/controllers/gclient"
	"github.com/clobrano/githubissues-operator/controllers/gclient/mock"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const expectedUrl = "https://github.com/clobrano/githubissues-operator"
const expectedIssueTitle = "Title of the issue"
const expectedIssueDescription = "some text describing the issue"

var _ = Describe("GithubissueController", func() {

	Context("default", func() {
		var underTest *v1alpha1.GithubIssue

		BeforeEach(func() {
			underTest = newGithubIssue(expectedIssueTitle, expectedIssueDescription)
			err := k8sClient.Create(context.Background(), underTest)
			Expect(err).NotTo(HaveOccurred())
		})

		AfterEach(func() {
			err := k8sClient.Delete(context.Background(), underTest)
			Expect(err).NotTo(HaveOccurred())
		})

		When("creating a resource", func() {
			It("it should have the expected values set", func() {
				Expect(underTest.Spec.Repo).To(Equal(expectedUrl))
				Expect(underTest.Spec.Title).To(Equal(expectedIssueTitle))
				Expect(underTest.Spec.Description).To(Equal(expectedIssueDescription))
			})
		})
	})

	Context("Reconciliation", func() {
		var (
			underTest *v1alpha1.GithubIssue
			myClient  client.WithWatch
			sch       *runtime.Scheme
			req       reconcile.Request
			mctrl     *gomock.Controller
			mgc       *mock.MockGithubClient
		)

		BeforeEach(func() {
			underTest = newGithubIssue(expectedIssueTitle, expectedIssueDescription)

			sch = scheme.Scheme
			sch.AddKnownTypes(v1alpha1.SchemeBuilder.GroupVersion, underTest)

			objs := []runtime.Object{underTest}
			myClient = fake.NewFakeClient(objs...)

			req = reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      "test",
					Namespace: "default",
				},
			}

			mctrl = gomock.NewController(GinkgoT())
			mgc = mock.NewMockGithubClient(mctrl)
		})

		AfterEach(func() {
			mctrl.Finish()
		})

		When("the issue does not exist", func() {
			It("should create it", func() {
				want := newExpectedGithubTicket()

				mgc.EXPECT().GetTickets(underTest.Spec.Repo).Return([]gclient.GithubTicket{
					{Title: "Title different than expected"},
				}, nil)
				mgc.EXPECT().CreateTicket(want).Return(nil)
				mgc.EXPECT().GetTickets(underTest.Spec.Repo).Return([]gclient.GithubTicket{want}, nil)
				mgc.EXPECT().IssueHasPR(want).Return(false)

				r := &GithubIssueReconciler{myClient, sch, mgc}
				_, err := r.Reconcile(context.TODO(), req)
				Expect(err).ToNot(HaveOccurred())
			})

			It("should return with error if it cannot create it", func() {
				want := newExpectedGithubTicket()

				mgc.EXPECT().GetTickets(underTest.Spec.Repo).Return([]gclient.GithubTicket{}, nil)
				mgc.EXPECT().CreateTicket(want).Return(fmt.Errorf("could not send Github API request"))

				r := &GithubIssueReconciler{myClient, sch, mgc}
				_, err := r.Reconcile(context.TODO(), req)
				Expect(err).To(HaveOccurred())
			})
		})

		When("the issue exists without the expected description", func() {
			It("should update the ticket description", func() {
				currentTicketHasWrongDescription := newExpectedGithubTicket()
				currentTicketHasWrongDescription.Number = 123
				currentTicketHasWrongDescription.Body = "a different issue description"
				mgc.EXPECT().GetTickets(underTest.Spec.Repo).Return([]gclient.GithubTicket{currentTicketHasWrongDescription}, nil)
				mgc.EXPECT().IssueHasPR(currentTicketHasWrongDescription)

				want := newExpectedGithubTicket()
				want.Number = 123
				mgc.EXPECT().UpdateTicket(want).Return(nil)

				r := &GithubIssueReconciler{myClient, sch, mgc}
				_, err := r.Reconcile(context.TODO(), req)
				Expect(err).ToNot(HaveOccurred())
			})

			It("should return an error if it cannot update the ticket description", func() {
				currentTicket := newExpectedGithubTicket()
				currentTicket.Number = 1
				currentTicket.Body = "a different issue description"

				mgc.EXPECT().GetTickets(underTest.Spec.Repo).Return([]gclient.GithubTicket{currentTicket}, nil)
				mgc.EXPECT().IssueHasPR(currentTicket)

				want := newExpectedGithubTicket()
				want.Number = 1
				mgc.EXPECT().UpdateTicket(want).Return(fmt.Errorf("could not send github API request"))

				r := &GithubIssueReconciler{myClient, sch, mgc}
				_, err := r.Reconcile(context.TODO(), req)
				Expect(err).To(HaveOccurred())
			})
		})

		When("the issue is linked and the title change in Github", func() {
			It("should not create a new ticket", func() {
				currentTicketIsUpToDate := newExpectedGithubTicket()
				currentTicketIsUpToDate.Number = 123

				mgc.EXPECT().GetTickets(underTest.Spec.Repo).Return([]gclient.GithubTicket{currentTicketIsUpToDate}, nil)
				mgc.EXPECT().IssueHasPR(currentTicketIsUpToDate)
				r := &GithubIssueReconciler{myClient, sch, mgc}

				_, err := r.Reconcile(context.TODO(), req)

				Expect(err).ToNot(HaveOccurred())
				Expect(myClient.Get(context.Background(), client.ObjectKeyFromObject(underTest), underTest)).To(Succeed())
				Expect(underTest.Status.TrackedIssueId).To(Equal(currentTicketIsUpToDate.Number))

				currentTicketWasChanged := currentTicketIsUpToDate
				currentTicketWasChanged.Title = "Title has changed"

				mgc.EXPECT().GetTickets(underTest.Spec.Repo).Return([]gclient.GithubTicket{currentTicketWasChanged}, nil)
				mgc.EXPECT().IssueHasPR(currentTicketWasChanged)
				// Expecting the ticket's title to be reverted back to Spec
				currentTicketWasChanged.Title = expectedIssueTitle
				mgc.EXPECT().UpdateTicket(currentTicketWasChanged).Return(nil)
				r = &GithubIssueReconciler{myClient, sch, mgc}
				_, err = r.Reconcile(context.TODO(), req)
				Expect(err).ToNot(HaveOccurred())
			})
		})

		When("the issue is open", func() {
			It("it should set corresponding open condition", func() {
				currentTicket := newExpectedGithubTicket()

				mgc.EXPECT().GetTickets(underTest.Spec.Repo).Return([]gclient.GithubTicket{currentTicket}, nil)
				mgc.EXPECT().IssueHasPR(currentTicket)

				r := &GithubIssueReconciler{myClient, sch, mgc}
				_, err := r.Reconcile(context.TODO(), req)
				Expect(err).ToNot(HaveOccurred())

				Expect(myClient.Get(context.Background(), client.ObjectKeyFromObject(underTest), underTest)).To(Succeed())
				Expect(underTest.Status.Conditions).To(ContainElement(
					And(
						HaveField("Type", "IsOpen"),
						HaveField("Status", metav1.ConditionTrue),
					)))
			})
		})

		When("the issue is closed", func() {
			It("it should set corresponding closed condition", func() {
				currentTicket := newExpectedGithubTicket()
				currentTicket.State = "closed"

				mgc.EXPECT().GetTickets(underTest.Spec.Repo).Return([]gclient.GithubTicket{currentTicket}, nil)
				mgc.EXPECT().IssueHasPR(currentTicket)

				r := &GithubIssueReconciler{myClient, sch, mgc}
				_, err := r.Reconcile(context.TODO(), req)
				Expect(err).ToNot(HaveOccurred())

				Expect(myClient.Get(context.Background(), client.ObjectKeyFromObject(underTest), underTest)).To(Succeed())
				Expect(underTest.Status.Conditions).To(ContainElement(
					And(
						HaveField("Type", "IsOpen"),
						HaveField("Status", metav1.ConditionFalse),
					)))
			})
		})

		When("the issue has a PR", func() {
			It("it should set corresponding HasPr condition", func() {
				currentTicket := newExpectedGithubTicket()

				mgc.EXPECT().GetTickets(underTest.Spec.Repo).Return([]gclient.GithubTicket{currentTicket}, nil)
				mgc.EXPECT().IssueHasPR(currentTicket).Return(true)

				r := &GithubIssueReconciler{myClient, sch, mgc}
				_, err := r.Reconcile(context.TODO(), req)
				Expect(err).ToNot(HaveOccurred())

				Expect(myClient.Get(context.Background(), client.ObjectKeyFromObject(underTest), underTest)).To(Succeed())
				Expect(underTest.Status.Conditions).To(ContainElement(
					And(
						HaveField("Type", "HasPr"),
						HaveField("Status", metav1.ConditionTrue),
					)))
			})
		})

		When("the issue has no PR", func() {
			It("it should unset corresponding HasPr condition", func() {
				currentTicket := newExpectedGithubTicket()

				mgc.EXPECT().GetTickets(underTest.Spec.Repo).Return([]gclient.GithubTicket{currentTicket}, nil)
				mgc.EXPECT().IssueHasPR(currentTicket).Return(false)

				r := &GithubIssueReconciler{myClient, sch, mgc}
				_, err := r.Reconcile(context.TODO(), req)
				Expect(err).ToNot(HaveOccurred())

				Expect(myClient.Get(context.Background(), client.ObjectKeyFromObject(underTest), underTest)).To(Succeed())
				Expect(underTest.Status.Conditions).To(ContainElement(
					And(
						HaveField("Type", "HasPr"),
						HaveField("Status", metav1.ConditionFalse),
					)))
			})
		})
	})

	Context("Resource deletion", func() {
		var (
			underTest *v1alpha1.GithubIssue
			myClient  client.WithWatch
			sch       *runtime.Scheme
			req       reconcile.Request
			mctrl     *gomock.Controller
			mgc       *mock.MockGithubClient
		)

		BeforeEach(func() {
			underTest = newGithubIssue(expectedIssueTitle, expectedIssueDescription)

			sch = scheme.Scheme
			sch.AddKnownTypes(v1alpha1.SchemeBuilder.GroupVersion, underTest)

			objs := []runtime.Object{underTest}
			myClient = fake.NewFakeClient(objs...)

			req = reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      "test",
					Namespace: "default",
				},
			}

			mctrl = gomock.NewController(GinkgoT())
			mgc = mock.NewMockGithubClient(mctrl)
		})

		AfterEach(func() {
			mctrl.Finish()
		})

		When("a ticket maching the CR Spec exists and it is open", func() {
			It("shall be closed", func() {
				ctx := context.Background()

				myClient.Create(ctx, underTest)

				r := &GithubIssueReconciler{myClient, sch, mgc}
				ticket := newExpectedGithubTicket()
				sameTicketButClosed := ticket
				sameTicketButClosed.State = "closed"

				mgc.EXPECT().GetTickets(underTest.Spec.Repo).Return([]gclient.GithubTicket{ticket}, nil).AnyTimes()
				mgc.EXPECT().IssueHasPR(ticket).Return(false).AnyTimes()

				_, err := r.Reconcile(context.TODO(), req)
				Expect(err).ToNot(HaveOccurred())

				Expect(myClient.Get(context.Background(), client.ObjectKeyFromObject(underTest), underTest)).To(Succeed())
				Expect(controllerutil.ContainsFinalizer(underTest, GIFinalizer)).To(BeTrue())

				mgc.EXPECT().UpdateTicket(sameTicketButClosed)
				myClient.Delete(ctx, underTest)

				_, err = r.Reconcile(context.TODO(), req)
				Expect(err).ToNot(HaveOccurred())
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
			Repo:        expectedUrl,
			Title:       title,
			Description: description,
		},
	}
}

func newExpectedGithubTicket() gclient.GithubTicket {
	return gclient.GithubTicket{
		Number:        0,
		Title:         expectedIssueTitle,
		Body:          expectedIssueDescription,
		State:         "open",
		RepositoryURL: expectedUrl,
		HasPr:         false,
	}
}
