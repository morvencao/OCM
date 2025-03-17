package services

import (
	"context"
	"strings"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	certificatev1 "k8s.io/api/certificates/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"

	certificatesv1informers "k8s.io/client-go/informers/certificates/v1"
	"k8s.io/client-go/kubernetes"
	certificatesv1listers "k8s.io/client-go/listers/certificates/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
	"open-cluster-management.io/ocm/pkg/server/services/codec"
	"open-cluster-management.io/sdk-go/pkg/cloudevents/generic/types"
	"open-cluster-management.io/sdk-go/pkg/cloudevents/server"
)

var csrDataType = types.CloudEventsDataType{
	Group:    certificatev1.GroupName,
	Version:  certificatev1.SchemeGroupVersion.Version,
	Resource: "certificatesigningrequests",
}

var _ server.Service = &CSRService{}

type CSRService struct {
	csrClient   kubernetes.Interface
	csrLister   certificatesv1listers.CertificateSigningRequestLister
	csrInformer certificatesv1informers.CertificateSigningRequestInformer
}

func NewCSRService(csrClient kubernetes.Interface, csrInformer certificatesv1informers.CertificateSigningRequestInformer) server.Service {
	return &CSRService{
		csrClient:   csrClient,
		csrLister:   csrInformer.Lister(),
		csrInformer: csrInformer,
	}
}

func (c *CSRService) Get(_ context.Context, resourceID string) (*cloudevents.Event, error) {
	csr, err := c.csrLister.Get(resourceID)
	if err != nil {
		return nil, err
	}

	evt, err := codec.ConvertObjectToEvent(csr)
	if err != nil {
		return nil, err
	}

	return evt, nil
}

func (c *CSRService) List(listOpts types.ListOptions) ([]*cloudevents.Event, error) {
	var evts []*cloudevents.Event
	if listOpts.Source != codec.Source {
		return evts, nil
	}

	// list csrs with prefix matching cluster name
	csrs, err := c.csrLister.List(labels.Everything())
	if err != nil {
		return nil, err
	}

	for _, csr := range csrs {
		if strings.HasPrefix(csr.Name, listOpts.ClusterName) {
			evt, err := codec.ConvertObjectToEvent(csr)
			if err != nil {
				return nil, err
			}
			evts = append(evts, evt)
		}
	}

	return evts, nil
}

// q if there is resourceVersion, this will return directly to the agent as conflict?
func (c *CSRService) HandleStatusUpdate(ctx context.Context, evt *cloudevents.Event) error {
	csr := &certificatev1.CertificateSigningRequest{}
	action, err := codec.ConvertEventToObject(evt, csr)
	if err != nil {
		return err
	}

	// only create and update action
	switch action {
	case createRequestAction:
		_, err := c.csrClient.CertificatesV1().CertificateSigningRequests().Create(ctx, csr, metav1.CreateOptions{})
		if err != nil {
			return err
		}
		// case updateStatusRequestAction:
		// 	_, err := c.csrClient.CertificatesV1().CertificateSigningRequests().UpdateStatus(ctx, csr, metav1.UpdateOptions{})
		// 	if err != nil {
		// 		return err
		// 	}
		// case updateRequestAction:
		// 	_, err := c.csrClient.CertificatesV1().CertificateSigningRequests().Update(ctx, csr, metav1.UpdateOptions{})
		// 	if err != nil {
		// 		return err
		// 	}
	}
	return nil
}

func (c *CSRService) RegisterHandler(handler server.EventHandler) {
	c.csrInformer.Informer().AddEventHandler(&cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			accessor, _ := meta.Accessor(obj)
			if err := handler.OnCreate(context.Background(), csrDataType, accessor.GetName()); err != nil {
				klog.Error(err)
			}
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			accessor, _ := meta.Accessor(newObj)
			if err := handler.OnUpdate(context.Background(), csrDataType, accessor.GetName()); err != nil {
				klog.Error(err)
			}
		},
		// agent does not need to care about delete event
	})
}
