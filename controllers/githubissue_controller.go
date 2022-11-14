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

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	trainingv1alpha1 "github.com/clobrano/githubissues-operator/api/v1alpha1"
)

type GithubTicket struct {
	Title, Description string
}

type GithubClient interface {
	GetTickets() ([]GithubTicket, error)
	CreateTicket(GithubTicket) error
	UpdateTicket(GithubTicket) error
}

// GithubIssueReconciler reconciles a GithubIssue object
type GithubIssueReconciler struct {
	client.Client
	Scheme     *runtime.Scheme
	RepoClient GithubClient
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
	ghi := &trainingv1alpha1.GithubIssue{}
	err := r.Get(ctx, req.NamespacedName, ghi)
	if err != nil {
		l.Error(err, "failed fetching GithubIssue resources", "object", ghi)
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}

		return ctrl.Result{}, err
	}

	tickets, err := r.RepoClient.GetTickets()
	if err != nil {
		return ctrl.Result{}, err
	}

	for _, t := range tickets {
		if t.Title != ghi.Spec.Title {
			continue
		}
		if t.Description != ghi.Spec.Description {
			r.RepoClient.UpdateTicket(GithubTicket{ghi.Spec.Title, ghi.Spec.Description})
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, nil
	}
	r.RepoClient.CreateTicket(GithubTicket{ghi.Spec.Title, ghi.Spec.Description})
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *GithubIssueReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&trainingv1alpha1.GithubIssue{}).
		Complete(r)
}
