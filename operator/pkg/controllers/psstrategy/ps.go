// Copyright 2022 The EasyDL Authors. All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package psstrategy

import (
	"context"
	elasticv1alpha1 "github.com/intelligent-machine-learning/easydl/operator/api/v1alpha1"
	controllers "github.com/intelligent-machine-learning/easydl/operator/pkg/controllers"
	logger "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"strconv"
)

const (
	psServicePort int = 3333
)

// PSManager generates a master pod object.
type PSManager struct {
	PSTaskManager
}

func init() {
	controllers.ReplicaManagers[ReplicaTypePS] = newPSManager()
}

func newPSManager() *PSManager {
	logger.Infof("init ps manager")
	return &PSManager{
		PSTaskManager: PSTaskManager{
			taskType: ReplicaTypePS,
		},
	}
}

// ReconcilePods creates a Pod on a K8s cluster
func (m *PSManager) ReconcilePods(
	r *controllers.ElasticJobReconciler,
	job *elasticv1alpha1.ElasticJob,
	resourceSpec *elasticv1alpha1.ReplicaResourceSpec,
) error {
	psStatus := m.getTaskStatus(job)
	currentNum := m.getTotalTaskCount(psStatus)
	aliveNum := int(psStatus.Active + psStatus.Pending)
	if resourceSpec.Replicas > aliveNum {
		m.scaleUpPS(r, job, currentNum, resourceSpec.Replicas-aliveNum)
	}
	return nil
}

func (m *PSManager) scaleUpPS(
	r *controllers.ElasticJobReconciler,
	job *elasticv1alpha1.ElasticJob,
	currentNum int,
	upNum int,
) error {
	cluster := m.getPSCluster(r.Client, job)
	for i := currentNum; i < currentNum+upNum; i++ {
		ps := m.newTask(job, i)
		m.insertTfConfigToEnv(&ps.Spec.Containers[0], cluster, i)
		err := r.Create(context.Background(), ps)
		if err != nil {
			r.Recorder.Eventf(
				job,
				corev1.EventTypeWarning,
				string(corev1.PodFailed),
				"PS pod %s created failed: %v",
				ps.Name,
				err)
			return err
			return err
		}
		service := m.newTaskService(job, i, psServicePort)
		err = r.Create(context.Background(), service)
		if err != nil {
			r.Recorder.Eventf(
				job,
				corev1.EventTypeWarning,
				string(corev1.PodFailed),
				"PS service %s created failed: %v",
				service.Name,
				err,
			)
			return err
		}
	}
	return nil
}

func (m *PSManager) getAllPSHosts(psPods []corev1.Pod, jobName string) []string {
	hosts := []string{}
	for _, pod := range psPods {
		psIndex, err := strconv.Atoi(pod.Labels[controllers.LabelReplicaIndexKey])
		if err != nil {
			continue
		}
		if pod.Status.Phase == corev1.PodPending || pod.Status.Phase == corev1.PodRunning {
			psServiceAddr := m.newTaskServiceAddr(jobName, psIndex, psServicePort)
			hosts = append(hosts, psServiceAddr)
		}
	}
	return hosts
}
