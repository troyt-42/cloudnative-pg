/*
Copyright The CloudNativePG Contributors

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

package status

import (
	"context"
	"reflect"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	apiv1 "github.com/cloudnative-pg/cloudnative-pg/api/v1"
	"github.com/cloudnative-pg/cloudnative-pg/pkg/management/log"
)

// RegisterPhase update phase in the status cluster with the
// proper reason
func RegisterPhase(
	ctx context.Context,
	cli client.Client,
	cluster *apiv1.Cluster,
	phase string,
	reason string,
) error {
	existingCluster := cluster.DeepCopy()
	return RegisterPhaseWithOrigCluster(ctx, cli, cluster, existingCluster, phase, reason)
}

// RegisterPhaseWithOrigCluster update phase in the status cluster with the
// proper reason, it also receives an origCluster to preserve other modifications done to the status
func RegisterPhaseWithOrigCluster(
	ctx context.Context,
	cli client.Client,
	modifiedCluster *apiv1.Cluster,
	origCluster *apiv1.Cluster,
	phase string,
	reason string,
) error {
	contextLogger := log.FromContext(ctx)
	// we ensure that the modifiedCluster conditions aren't nil before operating
	if modifiedCluster.Status.Conditions == nil {
		modifiedCluster.Status.Conditions = []metav1.Condition{}
	}

	modifiedCluster.Status.Phase = phase
	modifiedCluster.Status.PhaseReason = reason

	condition := metav1.Condition{
		Type:    string(apiv1.ConditionClusterReady),
		Status:  metav1.ConditionFalse,
		Reason:  string(apiv1.ClusterIsNotReady),
		Message: "Cluster Is Not Ready",
	}

	if modifiedCluster.Status.Phase == apiv1.PhaseHealthy {
		condition = metav1.Condition{
			Type:    string(apiv1.ConditionClusterReady),
			Status:  metav1.ConditionTrue,
			Reason:  string(apiv1.ClusterReady),
			Message: "Cluster is Ready",
		}
	}

	changed := meta.SetStatusCondition(&modifiedCluster.Status.Conditions, condition)
	contextLogger.Debug("SetStatusCondition", "condition", condition, "changed", changed)

	if !reflect.DeepEqual(origCluster, modifiedCluster) {
		if err := cli.Status().Patch(ctx, modifiedCluster, client.MergeFrom(origCluster)); err != nil {
			contextLogger.Error(err, "registerPhase, patched the status")
			return err
		}
		contextLogger.Debug("registerPhase, patched the status")
		cond := meta.FindStatusCondition(modifiedCluster.Status.Conditions, condition.Type)
		contextLogger.Debug("registerPhase, after patch, check", "condition", cond)
		var cl apiv1.Cluster
		err := cli.Get(ctx, types.NamespacedName{Namespace: modifiedCluster.Namespace, Name: modifiedCluster.Name}, &cl)
		if err != nil {
			contextLogger.Error(err, "registerPhase, checking the cluster object")
		}
		contextLogger.Debug("registerPhase, condition on cluster", "conditions", cl.Status.Conditions)
	} else {
		contextLogger.Debug("registerPhase, found no difference to apply")
	}

	return nil
}
