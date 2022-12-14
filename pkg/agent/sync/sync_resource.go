package sync

import (
	"encoding/json"
	"fmt"
	"github.com/choerodon/choerodon-cluster-agent/pkg/agent/channel"
	"github.com/choerodon/choerodon-cluster-agent/pkg/agent/model"
	agentnamespace "github.com/choerodon/choerodon-cluster-agent/pkg/agent/namespace"
	"github.com/choerodon/choerodon-cluster-agent/pkg/metrics"
	"github.com/choerodon/choerodon-cluster-agent/pkg/metrics/node"
	"github.com/golang/glog"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientset "k8s.io/client-go/kubernetes"
)

type Context struct {
	Namespaces *agentnamespace.Namespaces
	KubeClient clientset.Interface
	CrChan     *channel.CRChan
	StopCh     chan struct{}
	stopCh     chan struct{}
}

var syncFuncs []func(ctx *Context) error

func syncStatefulSet(ctx *Context) error {
	namespaces := ctx.Namespaces.GetAll()
	for _, ns := range namespaces {

		instances, err := ctx.KubeClient.AppsV1().StatefulSets(ns).List(metav1.ListOptions{})
		if err != nil {
			glog.Fatal("can not list resource, no rabc bind, exit !")
		} else {
			var podList []string
			for _, statefulset := range instances.Items {
				if statefulset.Labels[model.ReleaseLabel] != "" {
					podList = append(podList, statefulset.GetName())
				}
			}
			resourceList := &ResourceList{
				Resources:    podList,
				ResourceType: "StatefulSet",
			}
			content, err := json.Marshal(resourceList)
			if err != nil {
				glog.Fatal("marshal pod list error")
			} else {
				response := &model.Packet{
					Key:     fmt.Sprintf("env:%s", ns),
					Type:    model.ResourceSync,
					Payload: string(content),
				}
				ctx.CrChan.ResponseChan <- response
			}
		}
	}
	return nil
}

func syncReplicaSet(ctx *Context) error {
	namespaces := ctx.Namespaces.GetAll()
	for _, ns := range namespaces {

		rsList, err := ctx.KubeClient.ExtensionsV1beta1().ReplicaSets(ns).List(metav1.ListOptions{})
		if err != nil {
			glog.Fatal("can not list resource, no rabc bind, exit !")
		} else {
			var resourceSyncList []string
			for _, resource := range rsList.Items {
				if resource.Labels[model.ReleaseLabel] != "" {
					resourceSyncList = append(resourceSyncList, resource.GetName())
				}
			}
			resourceList := &ResourceList{
				Resources:    resourceSyncList,
				ResourceType: "ReplicaSet",
			}
			content, err := json.Marshal(resourceList)
			if err != nil {
				glog.Fatal("marshal ReplicaSet list error")
			} else {
				response := &model.Packet{
					Key:     fmt.Sprintf("env:%s", ns),
					Type:    model.ResourceSync,
					Payload: string(content),
				}
				ctx.CrChan.ResponseChan <- response
			}
		}
	}
	return nil
}

func syncService(ctx *Context) error {
	namespaces := ctx.Namespaces.GetAll()
	for _, ns := range namespaces {
		instances, err := ctx.KubeClient.CoreV1().Services(ns).List(metav1.ListOptions{})
		if err != nil {
			glog.Fatal(err)
		} else {
			var serviceList []string
			for _, instance := range instances.Items {
				if instance.Labels[model.ReleaseLabel] != "" {
					serviceList = append(serviceList, instance.GetName())
				}
			}
			resourceList := &ResourceList{
				Resources:    serviceList,
				ResourceType: "Service",
			}
			content, err := json.Marshal(resourceList)
			if err != nil {
				glog.Fatal("marshal service list error")
			} else {
				response := &model.Packet{
					Key:     fmt.Sprintf("env:%s", ns),
					Type:    model.ResourceSync,
					Payload: string(content),
				}
				ctx.CrChan.ResponseChan <- response
			}
		}
	}
	return nil
}

func syncPod(ctx *Context) error {
	namespaces := ctx.Namespaces.GetAll()
	for _, ns := range namespaces {

		pods, err := ctx.KubeClient.CoreV1().Pods(ns).List(metav1.ListOptions{})
		if err != nil {
			glog.Fatal("can not list resource, no rabc bind, exit !")
		} else {
			var podList []string
			for _, pod := range pods.Items {
				if pod.Labels[model.ReleaseLabel] != "" {
					podList = append(podList, pod.GetName())
				}
			}
			resourceList := &ResourceList{
				Resources:    podList,
				ResourceType: "Pod",
			}
			content, err := json.Marshal(resourceList)
			if err != nil {
				glog.Fatal("marshal pod list error")
			} else {
				response := &model.Packet{
					Key:     fmt.Sprintf("env:%s", ns),
					Type:    model.ResourceSync,
					Payload: string(content),
				}
				ctx.CrChan.ResponseChan <- response
			}
		}
	}
	return nil
}

func syncMetrics(ctx *Context) error {
	m := &node.Node{
		Client: ctx.KubeClient,
		CrChan: ctx.CrChan,
	}
	metrics.Register(m)
	return m.Run(ctx.stopCh)
}

func init() {
	syncFuncs = append(syncFuncs, syncStatefulSet)
	syncFuncs = append(syncFuncs, syncReplicaSet)
	syncFuncs = append(syncFuncs, syncService)
	syncFuncs = append(syncFuncs, syncPod)
	syncFuncs = append(syncFuncs, syncMetrics)
}

func Run(ctx *Context) {
	for _, fn := range syncFuncs {
		p := fn
		go func() {
			if err := p(ctx); err != nil {
				glog.Warningf("sync %v failed", fn)
			}
		}()
	}
	go func() {
		for {
			select {
			case <-ctx.StopCh:
				close(ctx.stopCh)
			case <-ctx.stopCh:
				return
			}
		}
	}()
}

var Y int

func (ctx *Context) ReSync() {
	if ctx.stopCh != nil {
		close(ctx.stopCh)
	}
	ctx.stopCh = make(chan struct{}, 1)
	Run(ctx)
}
