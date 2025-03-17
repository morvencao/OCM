package codec

import (
	"fmt"
	"strconv"

	"k8s.io/apimachinery/pkg/runtime"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"open-cluster-management.io/sdk-go/pkg/cloudevents/generic/types"
)

const (
	Source = "kube"
)

// Convert a cloudevent to a typed Kubernetes object
func ConvertEventToObject(evt *cloudevents.Event, out runtime.Object) (string, error) {
	eventType, err := types.ParseCloudEventsType(evt.Type())
	if err != nil {
		return "", fmt.Errorf("failed to parse cloud event type %s, %v", evt.Type(), err)
	}

	unstructuredObj := &unstructured.Unstructured{}
	if err := evt.DataAs(unstructuredObj); err != nil {
		return "", fmt.Errorf("failed to unmarshal event data %s, %v", string(evt.Data()), err)
	}

	if err := convertToTypedObject(unstructuredObj, out); err != nil {
		return "", fmt.Errorf("failed to convert unstructured object to typed object %v", err)
	}

	return string(eventType.Action), nil
}

// Convert a typed Kubernetes object to a cloudevent
func ConvertObjectToEvent(in runtime.Object) (*cloudevents.Event, error) {
	unstructuredObj, err := convertToUnstructured(in)
	if err != nil {
		return nil, fmt.Errorf("failed to convert object to unstructured %v", err)
	}

	resourceVersion, err := strconv.ParseInt(unstructuredObj.GetResourceVersion(), 10, 64)
	if err != nil {
		return nil, err
	}

	clusterName := unstructuredObj.GetNamespace()
	// for cluster resource, we use the name as the cluster name
	if clusterName == "" {
		clusterName = unstructuredObj.GetName()
	}
	evt := types.NewEventBuilder(Source, types.CloudEventsType{}).
		WithClusterName(clusterName).
		WithResourceID(unstructuredObj.GetName()).
		WithResourceVersion(resourceVersion).
		NewEvent()
	if !unstructuredObj.GetDeletionTimestamp().IsZero() {
		evt.SetExtension(types.ExtensionDeletionTimestamp, unstructuredObj.GetDeletionTimestamp().Time)
		return &evt, nil
	}

	// can we set the whole cluster resource into the data? what is the impact?
	if err := evt.SetData(cloudevents.ApplicationJSON, unstructuredObj); err != nil {
		return nil, fmt.Errorf("failed to encode object to a cloudevent: %v", err)
	}

	return &evt, nil
}

// Convert a typed Kubernetes object to *unstructured.Unstructured
func convertToUnstructured(obj runtime.Object) (*unstructured.Unstructured, error) {
	unstructuredMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
	if err != nil {
		return nil, err
	}

	unstructuredObj := &unstructured.Unstructured{Object: unstructuredMap}
	return unstructuredObj, nil
}

// Convert *unstructured.Unstructured to a typed Kubernetes object
func convertToTypedObject(obj *unstructured.Unstructured, out runtime.Object) error {
	return runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, out)
}
