package helm

import (
	"context"
	"encoding/json"
	"github.com/choerodon/choerodon-cluster-agent/pkg/agent/model"
	"github.com/choerodon/choerodon-cluster-agent/pkg/apis/choerodon/v1alpha1"
	"github.com/choerodon/choerodon-cluster-agent/pkg/helm"
	"github.com/choerodon/choerodon-cluster-agent/pkg/util/command"
	operatorutil "github.com/choerodon/choerodon-cluster-agent/pkg/util/operator"
	"github.com/golang/glog"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"strings"
)

// todo: maybe a wrong operator
func StartHelmRelease(opts *command.Opts, cmd *model.Packet) ([]*model.Packet, *model.Packet) {
	var req helm.StartReleaseRequest
	err := json.Unmarshal([]byte(cmd.Payload), &req)
	if err != nil {
		return nil, command.NewResponseError(cmd.Key, model.HelmReleaseStartFailed, err)
	}
	startResp, err := opts.HelmClient.StartRelease(&req)
	if err != nil {
		return nil, command.NewResponseError(cmd.Key, model.HelmReleaseStartFailed, err)
	}
	respB, err := json.Marshal(startResp)
	if err != nil {
		return nil, command.NewResponseError(cmd.Key, model.HelmReleaseStartFailed, err)
	}
	return nil, &model.Packet{
		Key:     cmd.Key,
		Type:    model.HelmReleaseStart,
		Payload: string(respB),
	}
}

func StopHelmRelease(opts *command.Opts, cmd *model.Packet) ([]*model.Packet, *model.Packet) {
	var req helm.StopReleaseRequest
	err := json.Unmarshal([]byte(cmd.Payload), &req)
	if err != nil {
		return nil, command.NewResponseError(cmd.Key, model.HelmReleaseStopFailed, err)
	}
	resp, err := opts.HelmClient.StopRelease(&req)
	if err != nil {
		return nil, command.NewResponseError(cmd.Key, model.HelmReleaseStopFailed, err)
	}
	respB, err := json.Marshal(resp)
	if err != nil {
		return nil, command.NewResponseError(cmd.Key, model.HelmReleaseStopFailed, err)
	}
	return nil, &model.Packet{
		Key:     cmd.Key,
		Type:    model.HelmReleaseStop,
		Payload: string(respB),
	}
}

func GetHelmReleaseContent(opts *command.Opts, cmd *model.Packet) ([]*model.Packet, *model.Packet) {
	var req helm.GetReleaseContentRequest
	err := json.Unmarshal([]byte(cmd.Payload), &req)
	if err != nil {
		return nil, command.NewResponseError(cmd.Key, model.HelmReleaseGetContentFailed, err)
	}
	resp, err := opts.HelmClient.GetReleaseContent(&req)
	if err != nil {
		return nil, command.NewResponseError(cmd.Key, model.HelmReleaseGetContentFailed, err)
	}
	if err != nil {
		return nil, command.NewResponseError(cmd.Key, model.HelmReleaseGetContentFailed, err)
	}
	respB, err := json.Marshal(resp)
	if err != nil {
		return nil, command.NewResponseError(cmd.Key, model.HelmReleaseGetContentFailed, err)
	}
	return nil, &model.Packet{
		Key:     cmd.Key,
		Type:    model.HelmReleaseGetContent,
		Payload: string(respB),
	}
}

func GetC7nHelmRelease(mgrs *operatorutil.MgrList, namespace string, releaseName string) (*v1alpha1.C7NHelmRelease, error) {

	client := mgrs.Get(namespace).GetClient()

	instance := &v1alpha1.C7NHelmRelease{}
	namespacedName := types.NamespacedName{
		Namespace: namespace,
		Name:      releaseName,
	}
	if err := client.Get(context.TODO(), namespacedName, instance); err != nil {
		if errors.IsNotFound(err) {
			return nil, err
		}
		return nil, nil
	}
	if instance.Annotations != nil && instance.Annotations[model.CommitLabel] != "" {
		return instance, nil
	}
	return nil, nil
}

func SyncStatus(opts *command.Opts, cmd *model.Packet) ([]*model.Packet, *model.Packet) {
	var reqs []helm.SyncRequest
	var reps = make([]*helm.SyncRequest, 0)

	err := json.Unmarshal([]byte(cmd.Payload), &reqs)
	if err != nil {
		glog.Errorf("unmarshal status sync failed %v", err)
		return nil, nil
	}

	kubeClient := opts.KubeClient
	helmClient := opts.HelmClient

	for _, syncRequest := range reqs {
		namespace := cmd.Namespace()
		switch syncRequest.ResourceType {
		case "ingress":
			commit, err := opts.KubeClient.GetIngress(namespace, syncRequest.ResourceName)
			if err != nil {
				reps = append(reps, newSyncResponse(syncRequest.ResourceName, syncRequest.ResourceType, "", syncRequest.Id))
			} else if commit != "" {
				reps = append(reps, newSyncResponse(syncRequest.ResourceName, syncRequest.ResourceType, commit, syncRequest.Id))
			}
			break
		case "service":
			commit, err := opts.KubeClient.GetService(namespace, syncRequest.ResourceName)
			if err != nil {
				reps = append(reps, newSyncResponse(syncRequest.ResourceName, syncRequest.ResourceType, "", syncRequest.Id))
			} else if commit != "" {
				reps = append(reps, newSyncResponse(syncRequest.ResourceName, syncRequest.ResourceType, commit, syncRequest.Id))
			}
			break
		case "certificate":
			commit, err := kubeClient.GetSecret(namespace, syncRequest.ResourceName)
			if err != nil {
				reps = append(reps, newSyncResponse(syncRequest.ResourceName, syncRequest.ResourceType, "", syncRequest.Id))
			} else if commit != "" {
				reps = append(reps, newSyncResponse(syncRequest.ResourceName, syncRequest.ResourceType, commit, syncRequest.Id))
			}
			break
		case "instance":
			chr, err := GetC7nHelmRelease(opts.Mgrs, namespace, syncRequest.ResourceName)
			if err != nil {
				reps = append(reps, newSyncResponse(syncRequest.ResourceName, syncRequest.ResourceType, "", syncRequest.Id))
			} else if chr != nil {
				if chr.Annotations[model.CommitLabel] == syncRequest.Commit {
					release, err := helmClient.GetRelease(&helm.GetReleaseContentRequest{ReleaseName: syncRequest.ResourceName})
					if err != nil {
						glog.Infof("release %s get error ", syncRequest.ResourceName, err)
						if strings.Contains(err.Error(), "not exist") {
							if kubeClient.IsReleaseJobRun(namespace, syncRequest.ResourceName) {
								glog.Errorf("release %s not exist and not job run ", syncRequest.ResourceName, err)
							} else {
								reps = append(reps, newSyncResponse(syncRequest.ResourceName, syncRequest.ResourceType, "", syncRequest.Id))
							}
						}
					}
					if release != nil && release.Status == "DEPLOYED" {
						if release.ChartVersion != chr.Spec.ChartVersion || release.Config != chr.Spec.Values {
							glog.Infof("release deployed but not consistent")
							reps = append(reps, newSyncResponse(syncRequest.ResourceName, syncRequest.ResourceType, "", syncRequest.Id))
						} else {
							reps = append(reps, newSyncResponse(syncRequest.ResourceName, syncRequest.ResourceType, syncRequest.Commit, syncRequest.Id))
						}
					}
				} else {
					reps = append(reps, newSyncResponse(syncRequest.ResourceName, syncRequest.ResourceType, syncRequest.Commit, syncRequest.Id))
				}
			}
			break
		}
	}

	if len(reps) == 0 {
		return nil, nil
	}

	respB, err := json.Marshal(reps)
	if err != nil {
		glog.Errorf("Marshal response error %v", err)
		return nil, nil
	}
	glog.Infof("sync response %s", string(respB))
	return nil, &model.Packet{
		Key:     cmd.Key,
		Type:    model.StatusSync,
		Payload: string(respB),
	}
}

func newSyncResponse(name string, reType string, commit string, id int32) *helm.SyncRequest {
	return &helm.SyncRequest{
		ResourceName: name,
		ResourceType: reType,
		Commit:       commit,
		Id:           id,
	}
}
