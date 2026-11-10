package kube

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestProblems(t *testing.T) {
	pods := []corev1.Pod{
		{
			ObjectMeta: metav1.ObjectMeta{Name: "pend", Namespace: "app"},
			Status: corev1.PodStatus{
				Phase: corev1.PodPending,
				Conditions: []corev1.PodCondition{{
					Type: corev1.PodScheduled, Status: corev1.ConditionFalse, Reason: "Unschedulable", Message: "0/1 nodes",
				}},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{Name: "crash", Namespace: "app"},
			Status: corev1.PodStatus{Phase: corev1.PodRunning, ContainerStatuses: []corev1.ContainerStatus{{
				Name: "app", RestartCount: 6,
				State:                corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: "CrashLoopBackOff", Message: "back-off"}},
				LastTerminationState: corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{Reason: "OOMKilled"}},
			}}},
		},
		{
			ObjectMeta: metav1.ObjectMeta{Name: "pull", Namespace: "app"},
			Status: corev1.PodStatus{ContainerStatuses: []corev1.ContainerStatus{{
				Name:  "app",
				State: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: "ImagePullBackOff"}},
			}}},
		},
	}
	got := Problems(pods)
	want := map[string]bool{"Unschedulable": false, "CrashLoopBackOff": false, "OOMKilled": false, "ImagePullBackOff": false}
	for _, p := range got {
		if _, ok := want[p.Reason]; ok {
			want[p.Reason] = true
		}
	}
	for reason, ok := range want {
		if !ok {
			t.Fatalf("faltou %s em %+v", reason, got)
		}
	}
}
