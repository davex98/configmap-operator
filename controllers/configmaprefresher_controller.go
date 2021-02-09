/*
Copyright 2021.

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
	"github.com/go-logr/logr"
	"github.com/google/uuid"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	refresherv1alpha1 "github.com/davex98/configmap-operator/api/v1alpha1"
)

// ConfigMapRefresherReconciler reconciles a ConfigMapRefresher object
type ConfigMapRefresherReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=refresher.burghardt.tech,resources=configmaprefreshers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=refresher.burghardt.tech,resources=configmaprefreshers/status,verbs=get;update;patch

func (r *ConfigMapRefresherReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	configMap := &v1.ConfigMap{}
	err := r.Get(ctx, req.NamespacedName, configMap)
	if err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}
	
	var refreshers refresherv1alpha1.ConfigMapRefresherList
	err = r.Client.List(ctx, &refreshers, &client.ListOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}
	for _, i := range refreshers.Items {
		if configMap.Name == i.Spec.ConfigMap {
			var pods v1.PodList
			err := r.Client.List(ctx, &pods, &client.ListOptions{Namespace: req.Namespace})
			if err != nil {
				r.Log.Error(err, "could not parse pods")
			}
			
			for _, p := range pods.Items {
				match := matchLabels(p.Labels, i.Spec.PodSelector)
				if match {
					err = r.replacePod(ctx, p)
					if err != nil {
						r.Log.Error(err, "could not replace pod")
						return ctrl.Result{}, err
					}
				}
			}
		}
	}
	return ctrl.Result{}, nil
}

func (r *ConfigMapRefresherReconciler) replacePod(ctx context.Context, p v1.Pod) error {
	newPod := p.DeepCopy()
	newPod.ResourceVersion = ""
	newPod.Name = uuid.New().String()

	err := r.Create(ctx, newPod, &client.CreateOptions{})
	if err != nil {
		return err
	}

	err = r.Delete(ctx, &p, &client.DeleteOptions{})
	if err != nil {
		return err
	}
	return nil
}

func matchLabels(a map[string]string, b map[string]string) bool {
	for ka, va := range a {
		for kb, vb := range b {
			if ka == kb && va == vb {
				return true
			}
		}
	}
	return false
}

func (r *ConfigMapRefresherReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1.ConfigMap{}).
		Complete(r)
}
