package metricsapiservice

import (
	"context"
	"math"
	"time"

	vclustercontext "github.com/loft-sh/vcluster/cmd/vcluster/context"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/rest"
	"k8s.io/klog"
	apiregistrationv1 "k8s.io/kube-aggregator/pkg/apis/apiregistration/v1"
	"k8s.io/metrics/pkg/apis/metrics"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	kerrors "k8s.io/apimachinery/pkg/api/errors"
	apiregclientv1 "k8s.io/kube-aggregator/pkg/client/clientset_generated/clientset/typed/apiregistration/v1"
)

const (
	MetricsVersion    = "v1beta1"
	MetricsAPIService = MetricsVersion + "." + metrics.GroupName // "v1beta1.metrics.k8s.io"

	KubernetesSvc = "kubernetes"
)

func checkExistingAPIService(ctx context.Context, kubeAggClient *apiregclientv1.ApiregistrationV1Client) bool {
	var exists bool

	_ = applyOperation(ctx, func(ctx context.Context) (bool, error) {
		_, err := kubeAggClient.APIServices().Get(ctx, MetricsAPIService, v1.GetOptions{})
		if err != nil {
			if kerrors.IsNotFound(err) {
				return true, nil
			}

			return false, err
		}

		exists = true
		return true, nil
	})

	return exists
}

func applyOperation(ctx context.Context, operationFunc wait.ConditionWithContextFunc) error {
	return wait.ExponentialBackoffWithContext(ctx, wait.Backoff{
		Duration: time.Second,
		Factor:   1.5,
		Cap:      time.Minute,
		Steps:    math.MaxInt32,
	}, operationFunc)
}

func deleteOperation(ctx context.Context, kubeAggClient *apiregclientv1.ApiregistrationV1Client) wait.ConditionWithContextFunc {
	return func(ctx context.Context) (bool, error) {
		err := kubeAggClient.APIServices().Delete(ctx, MetricsAPIService, v1.DeleteOptions{})
		if err != nil {
			if kerrors.IsNotFound(err) {
				return true, nil
			}

			return false, err
		}

		return true, nil
	}
}

func createOperation(ctx context.Context, kubeAggClient *apiregclientv1.ApiregistrationV1Client, client client.Client) wait.ConditionWithContextFunc {
	return func(ctx context.Context) (bool, error) {
		spec := apiregistrationv1.APIServiceSpec{
			Group:                metrics.GroupName,
			GroupPriorityMinimum: 100,
			Version:              MetricsVersion,
			VersionPriority:      100,
		}

		apiService := &apiregistrationv1.APIService{
			ObjectMeta: v1.ObjectMeta{
				Name: MetricsAPIService,
			},
		}

		_, err := controllerutil.CreateOrUpdate(ctx, client, apiService, func() error {
			apiService.Spec = spec
			return nil
		})
		if err != nil {
			if kerrors.IsAlreadyExists(err) {
				return true, nil
			}

			klog.Errorf("error creating api service %v", err)
			return false, err
		}

		return true, nil
	}
}

func RegisterOrDeregisterAPIService(ctx context.Context, options *vclustercontext.VirtualClusterOptions, vConfig *rest.Config) error {
	scheme := runtime.NewScheme()
	_ = apiregistrationv1.AddToScheme(scheme)

	kubeAggClient, err := apiregclientv1.NewForConfig(vConfig)
	if err != nil {
		return err
	}

	client, err := client.New(vConfig, client.Options{
		Scheme: scheme,
	})
	if err != nil {
		return err
	}

	exists := checkExistingAPIService(ctx, kubeAggClient)
	if options.ProxyMetricsServer {
		// register apiservice
		return applyOperation(ctx, createOperation(ctx, kubeAggClient, client))
	}

	if !options.ProxyMetricsServer && exists {
		// delete apiservice
		return applyOperation(ctx, deleteOperation(ctx, kubeAggClient))
	}

	return nil
}
