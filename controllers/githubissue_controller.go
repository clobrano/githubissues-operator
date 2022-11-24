/*
Copyright 2022.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	trainingv1alpha1 "github.com/clobrano/githubissues-operator/api/v1alpha1"
	"github.com/clobrano/githubissues-operator/controllers/gclient"
)

const GIFinalizer = "training.redhat.com/gifinalizer"

// GithubIssueReconciler reconciles a GithubIssue object
type GithubIssueReconciler struct {
	client.Client
	Scheme     *runtime.Scheme
	RepoClient gclient.GithubClient
}

//+kubebuilder:rbac:groups=training.redhat.com,resources=githubissues,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=training.redhat.com,resources=githubissues/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=training.redhat.com,resources=githubissues/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the GithubIssue object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.13.0/pkg/reconcile
func (r *GithubIssueReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	l := log.FromContext(ctx)
	gi := &trainingv1alpha1.GithubIssue{}
	err := r.Get(ctx, req.NamespacedName, gi)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}

		l.Error(err, "failed fetching GithubIssue resources", "object", gi)
		return ctrl.Result{}, err
	}

	isGithubIssueMarkedToBeDeleted := !gi.DeletionTimestamp.IsZero()
	giOrig := gi.DeepCopy()
	defer func() {
		if isGithubIssueMarkedToBeDeleted {
			return
		}
		mergeFrom := client.MergeFrom(giOrig)
		if streamBytes, err := mergeFrom.Data(gi); err != nil {
			return
		} else if string(streamBytes) == "{}" {
			return
		}
		err := r.Client.Status().Patch(ctx, gi, mergeFrom, &client.PatchOptions{})
		if err != nil {
			l.Error(err, "failed to patch Githubissue status")
			return
		}
	}()

	tickets, err := r.RepoClient.GetTickets(gi.Spec.Repo)
	if err != nil {
		l.Error(err, "failed to get tickets", "Repo URL", gi.Spec.Repo)
		return ctrl.Result{}, err
	}

	if len(tickets) == 0 {
		newTicket := gclient.GithubTicket{
			Number:        0,
			Title:         gi.Spec.Title,
			Body:          gi.Spec.Description,
			State:         "open",
			RepositoryURL: gi.Spec.Repo,
		}

		err = r.RepoClient.CreateTicket(newTicket)
		if err != nil {
			return ctrl.Result{}, err
		}
		l.Info("Reconcile", "Created new ticket", newTicket)
		return ctrl.Result{}, nil
	}

	for _, t := range tickets {
		if t.Title != gi.Spec.Title {
			continue
		}

		if t.State == "open" {
			meta.SetStatusCondition(&gi.Status.Conditions, metav1.Condition{
				Type:    "IsOpen",
				Status:  metav1.ConditionTrue,
				Reason:  "IssueIsOpen",
				Message: "GithubIssue operator detected that the issue is open",
			})
		} else {
			meta.SetStatusCondition(&gi.Status.Conditions, metav1.Condition{
				Type:    "IsOpen",
				Status:  metav1.ConditionFalse,
				Reason:  "IssueIsClosed",
				Message: "GithubIssue operator detected that the issue is closed",
			})
		}
		if r.RepoClient.IssueHasPR(t) {
			meta.SetStatusCondition(&gi.Status.Conditions, metav1.Condition{
				Type:    "HasPr",
				Status:  metav1.ConditionTrue,
				Reason:  "IssueHasPR",
				Message: "GithubIssue operator detected a PR linked to this issue",
			})
		} else {
			meta.SetStatusCondition(&gi.Status.Conditions, metav1.Condition{
				Type:    "HasPr",
				Status:  metav1.ConditionFalse,
				Reason:  "IssueDoesNotHavePR",
				Message: "GithubIssue operator detected no PR linked to this issue",
			})
		}

		if t.Body != gi.Spec.Description {
			t.Body = gi.Spec.Description
			err = r.RepoClient.UpdateTicket(t)
			if err != nil {
				return ctrl.Result{}, fmt.Errorf("could not update ticket: %v", err)
			}
			l.Info("Reconcile", "Updated ticket", t.Number)
		}

		if t.State == "open" && isGithubIssueMarkedToBeDeleted {
			t.State = "closed"
			err = r.RepoClient.UpdateTicket(t)
			if err != nil {
				return ctrl.Result{}, fmt.Errorf("could not close ticket: %v", err)
			}
			controllerutil.RemoveFinalizer(gi, GIFinalizer)
			err = r.Update(ctx, gi)
			if err != nil {
				return ctrl.Result{}, fmt.Errorf("could not remove finalizer: %v", err)
			}
			return ctrl.Result{}, nil
		}
	}

	// Add finalizer
	if !controllerutil.ContainsFinalizer(gi, GIFinalizer) {
		controllerutil.AddFinalizer(gi, GIFinalizer)
		err = r.Update(ctx, gi)
		if err != nil {
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *GithubIssueReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&trainingv1alpha1.GithubIssue{}).
		Complete(r)
}
