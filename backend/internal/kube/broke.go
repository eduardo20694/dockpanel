package kube

import (
	"strings"

	corev1 "k8s.io/api/core/v1"
)

type Problem struct {
	Namespace string `json:"namespace"`
	Pod       string `json:"pod"`
	Container string `json:"container,omitempty"`
	Reason    string `json:"reason"`
	Message   string `json:"message,omitempty"`
	Severity  string `json:"severity"`
}

// Problems classifica CrashLoopBackOff, ImagePullBackOff, OOMKilled e Pending.
func Problems(pods []corev1.Pod) []Problem {
	var out []Problem
	for _, p := range pods {
		if p.Status.Phase == corev1.PodPending {
			msg := p.Status.Message
			reason := "Pending"
			for _, c := range p.Status.Conditions {
				if c.Type == corev1.PodScheduled && c.Status == corev1.ConditionFalse && c.Reason != "" {
					reason = c.Reason
					msg = c.Message
				}
			}
			out = append(out, Problem{Namespace: p.Namespace, Pod: p.Name, Reason: reason, Message: msg, Severity: "warning"})
		}
		for _, cs := range p.Status.ContainerStatuses {
			if cs.State.Waiting != nil {
				reason := cs.State.Waiting.Reason
				if reason == "CrashLoopBackOff" || reason == "ImagePullBackOff" || reason == "ErrImagePull" || reason == "CreateContainerConfigError" {
					sev := "critical"
					if reason == "ImagePullBackOff" || reason == "ErrImagePull" {
						sev = "warning"
					}
					out = append(out, Problem{Namespace: p.Namespace, Pod: p.Name, Container: cs.Name, Reason: reason, Message: cs.State.Waiting.Message, Severity: sev})
				}
			}
			if cs.State.Terminated != nil && (cs.State.Terminated.Reason == "OOMKilled" || cs.State.Terminated.ExitCode == 137) {
				out = append(out, Problem{Namespace: p.Namespace, Pod: p.Name, Container: cs.Name, Reason: "OOMKilled", Message: cs.State.Terminated.Message, Severity: "critical"})
			}
			if cs.LastTerminationState.Terminated != nil && cs.LastTerminationState.Terminated.Reason == "OOMKilled" {
				out = append(out, Problem{Namespace: p.Namespace, Pod: p.Name, Container: cs.Name, Reason: "OOMKilled", Message: "último término", Severity: "critical"})
			}
			if cs.RestartCount >= 5 && !hasReason(out, p.Name, "CrashLoopBackOff") {
				out = append(out, Problem{Namespace: p.Namespace, Pod: p.Name, Container: cs.Name, Reason: "restarts", Message: strings.TrimSpace(reasonOf(cs)), Severity: "warning"})
			}
		}
	}
	if out == nil {
		out = []Problem{}
	}
	return out
}

func hasReason(list []Problem, pod, reason string) bool {
	for _, p := range list {
		if p.Pod == pod && p.Reason == reason {
			return true
		}
	}
	return false
}

func reasonOf(cs corev1.ContainerStatus) string {
	if cs.State.Waiting != nil {
		return cs.State.Waiting.Reason
	}
	if cs.State.Terminated != nil {
		return cs.State.Terminated.Reason
	}
	return ""
}
