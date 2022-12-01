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

package v1alpha1

import (
	"fmt"
	"net/http"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

// log is for logging in this package.
var githubissuelog = logf.Log.WithName("githubissue-resource")

func (r *GithubIssue) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

// TODO(user): EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!

//+kubebuilder:webhook:path=/mutate-training-redhat-com-v1alpha1-githubissue,mutating=true,failurePolicy=fail,sideEffects=None,groups=training.redhat.com,resources=githubissues,verbs=create;update,versions=v1alpha1,name=mgithubissue.kb.io,admissionReviewVersions=v1

var _ webhook.Defaulter = &GithubIssue{}

// Default implements webhook.Defaulter so a webhook will be registered for the type
func (r *GithubIssue) Default() {
	githubissuelog.Info("default", "name", r.Name)

	// TODO(user): fill in your defaulting logic.
}

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
//+kubebuilder:webhook:path=/validate-training-redhat-com-v1alpha1-githubissue,mutating=false,failurePolicy=fail,sideEffects=None,groups=training.redhat.com,resources=githubissues,verbs=create;update,versions=v1alpha1,name=vgithubissue.kb.io,admissionReviewVersions=v1

var _ webhook.Validator = &GithubIssue{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *GithubIssue) ValidateCreate() error {
	githubissuelog.Info("validate create", "name", r.Name)

	return r.validateRepo()
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *GithubIssue) ValidateUpdate(old runtime.Object) error {
	githubissuelog.Info("validate update", "name", r.Name)

	// TODO(user): fill in your validation logic upon object update.
	return nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *GithubIssue) ValidateDelete() error {
	githubissuelog.Info("validate delete", "name", r.Name)

	// TODO(user): fill in your validation logic upon object deletion.
	return nil
}

func (r *GithubIssue) validateRepo() error {
	if rsp, err := http.Get(r.Spec.Repo); err != nil || rsp.StatusCode != http.StatusOK {
		githubissuelog.Info("Repo URL validation", "err", err, "Status", rsp.Status)
		return fmt.Errorf("Repo %v is unreachable", r.Spec.Repo)
	}
	return nil
}
