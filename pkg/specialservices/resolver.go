package specialservices

import (
	synccontext "github.com/loft-sh/vcluster/pkg/controllers/syncer/context"
	"github.com/loft-sh/vcluster/pkg/setup/options"
	"github.com/loft-sh/vcluster/pkg/util/translate"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var Default Interface

func SetDefault(ctrlCtx *options.VirtualClusterOptions) {
	Default = defaultNameserverFinder(ctrlCtx.IsK8sDistro)
}

const (
	DefaultKubeDNSServiceName      = "kube-dns"
	DefaultKubeDNSServiceNamespace = "kube-system"
)

type SpecialServiceSyncer func(
	ctx *synccontext.SyncContext,
	svcNamespace,
	svcName string,
	vSvcToSync types.NamespacedName,
	servicePortTranslator ServicePortTranslator,
) error

type Interface interface {
	SpecialServicesToSync() map[types.NamespacedName]SpecialServiceSyncer
	DNSNamespace(ctx *synccontext.SyncContext) (client.Client, string)
}

type NameserverFinder struct {
	SpecialServices map[types.NamespacedName]SpecialServiceSyncer
}

func (f *NameserverFinder) DNSNamespace(ctx *synccontext.SyncContext) (client.Client, string) {
	return ctx.PhysicalClient, translate.Default.PhysicalNamespace(DefaultKubeDNSServiceNamespace)
}

func (f *NameserverFinder) SpecialServicesToSync() map[types.NamespacedName]SpecialServiceSyncer {
	return f.SpecialServices
}

func defaultNameserverFinder(k8sDistro bool) Interface {
	specialServicesMap := map[types.NamespacedName]SpecialServiceSyncer{
		DefaultKubernetesSvcKey: SyncKubernetesService,
	}

	if k8sDistro {
		specialServicesMap[VclusterProxyMetricsSvcKey] = SyncVclusterProxyService
	}

	return &NameserverFinder{
		SpecialServices: specialServicesMap,
	}
}
