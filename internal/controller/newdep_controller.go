/*
Copyright 2024.

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

package controller

import (
	"context"
	"fmt"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	appsv1beta1 "github.com/haowenj/newcrd-api/api/v1beta1"
	k8sappsv1 "k8s.io/api/apps/v1"
	k8scorev1 "k8s.io/api/core/v1"
)

// NewDepReconciler reconciles a NewDep object
type NewDepReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

// +kubebuilder:rbac:groups=apps.newcrd.com,resources=newdeps,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps.newcrd.com,resources=newdeps/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=apps.newcrd.com,resources=newdeps/finalizers,verbs=update
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
func (r *NewDepReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logs := log.FromContext(ctx)
	newcrd := &appsv1beta1.NewDep{}
	dep := &k8sappsv1.Deployment{}
	err := r.Get(ctx, req.NamespacedName, newcrd)
	if err != nil {
		if !errors.IsNotFound(err) {
			r.Recorder.Event(newcrd, k8scorev1.EventTypeWarning, "FailedGetNewcrd", err.Error())
		}
		return ctrl.Result{}, nil
	}
	err = r.Get(ctx, req.NamespacedName, dep)
	if err != nil {
		//如果查不到这个deployment就去创建
		if errors.IsNotFound(err) {
			logs.Info("Deployment Not Found " + req.NamespacedName.Name)
			deploy := createDeployment(newcrd)
			//binding deployment to podsbook
			// 调用SetControllerReference方法只是往被控制对象的结构体里加入了关联信息，并没有更新到etcd里，需要调用create或者update方法更新etcd里的数据才能起效果
			if err = ctrl.SetControllerReference(newcrd, deploy, r.Scheme); err != nil {
				return ctrl.Result{}, err
			}
			err = r.Create(ctx, deploy)
			if err != nil {
				r.Recorder.Event(newcrd, k8scorev1.EventTypeWarning, "FailedCreateDeployment", err.Error())
				return ctrl.Result{}, err
			}
			r.Recorder.Event(newcrd, k8scorev1.EventTypeNormal, "SuccessCreateDeployment", fmt.Sprintf("success create deployment: %s", req.NamespacedName.Name))
			//status是子资源，更新的时候使用Status接口，直接调用update方法更新podsbook对象不起作用
			newcrd.Status.RealReplica = *newcrd.Spec.Replica
			err = r.Status().Update(ctx, newcrd)
			if err != nil {
				r.Recorder.Event(newcrd, k8scorev1.EventTypeWarning, "FailedUpdateStatus", err.Error())
				return ctrl.Result{}, err
			}
		}
		return ctrl.Result{}, err
	}
	//如果查到了看下是否需要更新
	if newcrd.Status.RealReplica != *newcrd.Spec.Replica {
		newcrd.Status.RealReplica = *newcrd.Spec.Replica
		err = r.Status().Update(ctx, newcrd)
		if err != nil {
			r.Recorder.Event(newcrd, k8scorev1.EventTypeWarning, "FailedUpdateStatus", err.Error())
			return ctrl.Result{}, err
		}
	}

	if *dep.Spec.Replicas != *newcrd.Spec.Replica || dep.Spec.Template.Spec.Containers[0].Image != *newcrd.Spec.Image {
		dep.Spec.Replicas = newcrd.Spec.Replica
		dep.Spec.Template.Spec.Containers[0].Image = *newcrd.Spec.Image
		err = r.Update(ctx, dep)
		if err != nil {
			r.Recorder.Event(newcrd, k8scorev1.EventTypeWarning, "FailedUpdateDeployment", err.Error())
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *NewDepReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&appsv1beta1.NewDep{}).
		Owns(&k8sappsv1.Deployment{}).
		Complete(r)
}

func createDeployment(newcrd *appsv1beta1.NewDep) *k8sappsv1.Deployment {
	deployment := &k8sappsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: newcrd.Namespace,
			Name:      newcrd.Name,
		},
		Spec: k8sappsv1.DeploymentSpec{
			Replicas: newcrd.Spec.Replica,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": newcrd.Name,
				},
			},

			Template: k8scorev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": newcrd.Name,
					},
				},
				Spec: k8scorev1.PodSpec{
					Containers: []k8scorev1.Container{
						{
							Name:  newcrd.Name,
							Image: *newcrd.Spec.Image,
							Ports: []k8scorev1.ContainerPort{
								{
									Name:          newcrd.Name,
									Protocol:      k8scorev1.ProtocolTCP,
									ContainerPort: 80,
								},
							},
						},
					},
				},
			},
		},
	}
	return deployment
}
