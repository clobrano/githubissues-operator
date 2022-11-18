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
	"encoding/base64"
	"fmt"
	"os"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	trainingv1alpha1 "github.com/clobrano/githubissues-operator/api/v1alpha1"
	"github.com/clobrano/githubissues-operator/controllers/gclient"
	corev1 "k8s.io/api/core/v1"
)

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
		l.Error(err, "failed fetching GithubIssue resources", "object", gi)
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}

		return ctrl.Result{}, err
	}

	giOrig := gi.DeepCopy()
	defer func() {
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

	err = setGithubTokenEnvFromSecret(r.Client)
	if err != nil {
		return ctrl.Result{}, err
	}
	tickets, err := r.RepoClient.GetTickets(gi.Spec.Repo)
	if err != nil {
		l.Error(err, "failed to get tickets", "Repo URL", gi.Spec.Repo)
		return ctrl.Result{}, err
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
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, nil
	}

	newTicket := gclient.GithubTicket{
		Number:        0,
		Title:         gi.Spec.Title,
		Body:          gi.Spec.Description,
		State:         "open",
		RepositoryURL: gi.Spec.Repo,
	}

	err = r.RepoClient.CreateTicket(newTicket)
	if err != nil {
		l.Error(err, "could not create ticket", newTicket)
	}
	return ctrl.Result{}, err
}

// SetupWithManager sets up the controller with the Manager.
func (r *GithubIssueReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&trainingv1alpha1.GithubIssue{}).
		Complete(r)
}

func setGithubTokenEnvFromSecret(client client.Client) error {
	secret := &corev1.Secret{}
	err := client.Get(context.TODO(), types.NamespacedName{Namespace: "default", Name: "gh-token-secret"}, secret)
	if err != nil {
		return fmt.Errorf("could not get secret: %v", err)
	}
	enc := base64.StdEncoding.EncodeToString(secret.Data["GITHUB_TOKEN"])
	token, err := base64.StdEncoding.DecodeString(enc)
	if err != nil {
		return fmt.Errorf("could not decode secret: %v", err)
	}
	os.Setenv("GITHUB_TOKEN", string(token))
	return nil
}
