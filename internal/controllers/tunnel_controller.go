/*
MIT License

Copyright (c) 2022 ngrok, Inc.

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
*/

package controllers

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	ingressv1alpha1 "github.com/ngrok/kubernetes-ingress-controller/api/v1alpha1"
	"github.com/ngrok/kubernetes-ingress-controller/pkg/tunneldriver"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

// TunnelReconciler reconciles a Tunnel object
type TunnelReconciler struct {
	client.Client

	Log          logr.Logger
	Scheme       *runtime.Scheme
	Recorder     record.EventRecorder
	TunnelDriver *tunneldriver.TunnelDriver

	controller *baseController[*ingressv1alpha1.Tunnel]
}

// SetupWithManager sets up the controller with the Manager
func (r *TunnelReconciler) SetupWithManager(mgr ctrl.Manager) error {
	var err error

	if r.TunnelDriver == nil {
		return fmt.Errorf("TunnelDriver is nil")
	}

	r.controller = &baseController[*ingressv1alpha1.Tunnel]{
		Kube:     r.Client,
		Log:      r.Log,
		Recorder: r.Recorder,

		kubeType: "v1alpha1.Tunnel",
		update:   r.update,
		delete:   r.delete,
	}

	cont, err := controller.NewUnmanaged("tunnel-controller", mgr, controller.Options{
		Reconciler: r,
		LogConstructor: func(_ *reconcile.Request) logr.Logger {
			return r.Log
		},
	})
	if err != nil {
		return err
	}

	cont = NonLeaderElectedController{cont}

	if err := cont.Watch(
		&source.Kind{Type: &ingressv1alpha1.Tunnel{}},
		&handler.EnqueueRequestForObject{},
		commonPredicateFilters,
	); err != nil {
		return err
	}

	return mgr.Add(cont)
}

//+kubebuilder:rbac:groups=ingress.k8s.ngrok.com,resources=tunnels,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=ingress.k8s.ngrok.com,resources=tunnels/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=ingress.k8s.ngrok.com,resources=tunnels/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.13.1/pkg/reconcile
func (r *TunnelReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	return r.controller.reconcile(ctx, req, new(ingressv1alpha1.Tunnel))
}

func (r *TunnelReconciler) update(ctx context.Context, tunnel *ingressv1alpha1.Tunnel) error {
	tunnelName := fmt.Sprintf("%s/%s", tunnel.Namespace, tunnel.Name)
	return r.TunnelDriver.CreateTunnel(ctx, tunnelName, tunnel.Spec.Labels, tunnel.Spec.BackendConfig, tunnel.Spec.ForwardsTo)
}

func (r *TunnelReconciler) delete(ctx context.Context, tunnel *ingressv1alpha1.Tunnel) error {
	tunnelName := fmt.Sprintf("%s/%s", tunnel.Namespace, tunnel.Name)
	return r.TunnelDriver.DeleteTunnel(ctx, tunnelName)
}
