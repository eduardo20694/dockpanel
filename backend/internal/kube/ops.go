package kube

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/remotecommand"
)

type NamespaceView struct {
	Name   string `json:"name"`
	Status string `json:"status"`
}

type WorkloadView struct {
	Kind      string `json:"kind"`
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
	Replicas  int32  `json:"replicas"`
	Ready     int32  `json:"ready"`
	Image     string `json:"image,omitempty"`
}

type PodView struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
	Phase     string `json:"phase"`
	Node      string `json:"node,omitempty"`
	Restarts  int32  `json:"restarts"`
	Ready     string `json:"ready"`
}

type ServiceView struct {
	Namespace string   `json:"namespace"`
	Name      string   `json:"name"`
	Type      string   `json:"type"`
	ClusterIP string   `json:"clusterIP"`
	Ports     []string `json:"ports"`
}

type IngressView struct {
	Namespace string   `json:"namespace"`
	Name      string   `json:"name"`
	Hosts     []string `json:"hosts"`
	Class     string   `json:"class,omitempty"`
}

type NamedKeys struct {
	Namespace string   `json:"namespace"`
	Name      string   `json:"name"`
	Keys      []string `json:"keys"`
}

func (c *Cluster) Namespaces(ctx context.Context) ([]NamespaceView, error) {
	if err := c.ready(); err != nil {
		return nil, err
	}
	list, err := c.Client.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	out := make([]NamespaceView, 0, len(list.Items))
	for _, n := range list.Items {
		out = append(out, NamespaceView{Name: n.Name, Status: string(n.Status.Phase)})
	}
	return out, nil
}

func (c *Cluster) Workloads(ctx context.Context, namespace string) ([]WorkloadView, error) {
	if err := c.ready(); err != nil {
		return nil, err
	}
	ns := nsOrAll(namespace)
	var out []WorkloadView
	deps, err := c.Client.AppsV1().Deployments(ns).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	for _, d := range deps.Items {
		out = append(out, workload("Deployment", d.Namespace, d.Name, replicas(d.Spec.Replicas), d.Status.ReadyReplicas, images(d.Spec.Template.Spec)))
	}
	sts, err := c.Client.AppsV1().StatefulSets(ns).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	for _, s := range sts.Items {
		out = append(out, workload("StatefulSet", s.Namespace, s.Name, replicas(s.Spec.Replicas), s.Status.ReadyReplicas, images(s.Spec.Template.Spec)))
	}
	dss, err := c.Client.AppsV1().DaemonSets(ns).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	for _, d := range dss.Items {
		out = append(out, workload("DaemonSet", d.Namespace, d.Name, d.Status.DesiredNumberScheduled, d.Status.NumberReady, images(d.Spec.Template.Spec)))
	}
	if out == nil {
		out = []WorkloadView{}
	}
	return out, nil
}

func (c *Cluster) Pods(ctx context.Context, namespace string) ([]PodView, error) {
	if err := c.ready(); err != nil {
		return nil, err
	}
	list, err := c.Client.CoreV1().Pods(nsOrAll(namespace)).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	out := make([]PodView, 0, len(list.Items))
	for _, p := range list.Items {
		ready, total := 0, len(p.Status.ContainerStatuses)
		var restarts int32
		for _, cs := range p.Status.ContainerStatuses {
			restarts += cs.RestartCount
			if cs.Ready {
				ready++
			}
		}
		out = append(out, PodView{
			Namespace: p.Namespace, Name: p.Name, Phase: string(p.Status.Phase), Node: p.Spec.NodeName,
			Restarts: restarts, Ready: fmt.Sprintf("%d/%d", ready, total),
		})
	}
	return out, nil
}

func (c *Cluster) PodLogs(ctx context.Context, namespace, name, containerName string, tail int64) (string, error) {
	if err := c.ready(); err != nil {
		return "", err
	}
	if tail <= 0 {
		tail = 200
	}
	opts := &corev1.PodLogOptions{TailLines: &tail}
	if containerName != "" {
		opts.Container = containerName
	}
	rc, err := c.Client.CoreV1().Pods(namespace).GetLogs(name, opts).Stream(ctx)
	if err != nil {
		return "", err
	}
	defer rc.Close()
	b, err := io.ReadAll(io.LimitReader(rc, 1<<20))
	return string(b), err
}

func (c *Cluster) Exec(ctx context.Context, namespace, name string, command []string) (string, error) {
	if err := c.ready(); err != nil {
		return "", err
	}
	if c.Config == nil {
		return "", fmt.Errorf("exec exige a configuração REST do cluster")
	}
	if len(command) == 0 {
		return "", fmt.Errorf("comando vazio")
	}
	req := c.Client.CoreV1().RESTClient().Post().Resource("pods").Name(name).Namespace(namespace).SubResource("exec")
	req.VersionedParams(&corev1.PodExecOptions{Command: command, Stdout: true, Stderr: true}, scheme.ParameterCodec)
	exec, err := remotecommand.NewSPDYExecutor(c.Config, "POST", req.URL())
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	err = exec.StreamWithContext(ctx, remotecommand.StreamOptions{Stdout: &buf, Stderr: &buf})
	return buf.String(), err
}

func (c *Cluster) RestartPod(ctx context.Context, namespace, name string) error {
	if err := c.ready(); err != nil {
		return err
	}
	return c.Client.CoreV1().Pods(namespace).Delete(ctx, name, metav1.DeleteOptions{})
}

func (c *Cluster) Services(ctx context.Context, namespace string) ([]ServiceView, error) {
	if err := c.ready(); err != nil {
		return nil, err
	}
	list, err := c.Client.CoreV1().Services(nsOrAll(namespace)).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	out := make([]ServiceView, 0, len(list.Items))
	for _, s := range list.Items {
		v := ServiceView{Namespace: s.Namespace, Name: s.Name, Type: string(s.Spec.Type), ClusterIP: s.Spec.ClusterIP, Ports: []string{}}
		for _, p := range s.Spec.Ports {
			v.Ports = append(v.Ports, fmt.Sprintf("%d/%s", p.Port, p.Protocol))
		}
		out = append(out, v)
	}
	return out, nil
}

func (c *Cluster) Ingresses(ctx context.Context, namespace string) ([]IngressView, error) {
	if err := c.ready(); err != nil {
		return nil, err
	}
	list, err := c.Client.NetworkingV1().Ingresses(nsOrAll(namespace)).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	out := make([]IngressView, 0, len(list.Items))
	for _, ing := range list.Items {
		v := IngressView{Namespace: ing.Namespace, Name: ing.Name, Hosts: []string{}}
		if ing.Spec.IngressClassName != nil {
			v.Class = *ing.Spec.IngressClassName
		}
		for _, r := range ing.Spec.Rules {
			if r.Host != "" {
				v.Hosts = append(v.Hosts, r.Host)
			}
		}
		out = append(out, v)
	}
	return out, nil
}

func (c *Cluster) ConfigMaps(ctx context.Context, namespace string) ([]NamedKeys, error) {
	if err := c.ready(); err != nil {
		return nil, err
	}
	list, err := c.Client.CoreV1().ConfigMaps(nsOrAll(namespace)).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	out := make([]NamedKeys, 0, len(list.Items))
	for _, cm := range list.Items {
		keys := make([]string, 0, len(cm.Data))
		for k := range cm.Data {
			keys = append(keys, k)
		}
		out = append(out, NamedKeys{Namespace: cm.Namespace, Name: cm.Name, Keys: keys})
	}
	return out, nil
}

func (c *Cluster) Secrets(ctx context.Context, namespace string) ([]NamedKeys, error) {
	if err := c.ready(); err != nil {
		return nil, err
	}
	list, err := c.Client.CoreV1().Secrets(nsOrAll(namespace)).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	out := make([]NamedKeys, 0, len(list.Items))
	for _, s := range list.Items {
		keys := make([]string, 0, len(s.Data))
		for k := range s.Data {
			keys = append(keys, k)
		}
		out = append(out, NamedKeys{Namespace: s.Namespace, Name: s.Name, Keys: keys})
	}
	return out, nil
}

func (c *Cluster) Scale(ctx context.Context, kind, namespace, name string, replicas int32) error {
	if err := c.ready(); err != nil {
		return err
	}
	return retryConflict(func() error {
		return c.scaleOnce(ctx, kind, namespace, name, replicas)
	})
}

func (c *Cluster) scaleOnce(ctx context.Context, kind, namespace, name string, replicas int32) error {
	switch kind {
	case "", "Deployment", "deployment":
		sc, err := c.Client.AppsV1().Deployments(namespace).GetScale(ctx, name, metav1.GetOptions{})
		if err != nil {
			return err
		}
		sc.Spec.Replicas = replicas
		_, err = c.Client.AppsV1().Deployments(namespace).UpdateScale(ctx, name, sc, metav1.UpdateOptions{})
		return err
	case "StatefulSet", "statefulset":
		sc, err := c.Client.AppsV1().StatefulSets(namespace).GetScale(ctx, name, metav1.GetOptions{})
		if err != nil {
			return err
		}
		sc.Spec.Replicas = replicas
		_, err = c.Client.AppsV1().StatefulSets(namespace).UpdateScale(ctx, name, sc, metav1.UpdateOptions{})
		return err
	default:
		return fmt.Errorf("scale não suportado para %s", kind)
	}
}

func (c *Cluster) RolloutRestart(ctx context.Context, kind, namespace, name string) error {
	if err := c.ready(); err != nil {
		return err
	}
	return retryConflict(func() error {
		return c.rolloutOnce(ctx, kind, namespace, name)
	})
}

func (c *Cluster) rolloutOnce(ctx context.Context, kind, namespace, name string) error {
	stamp := time.Now().UTC().Format(time.RFC3339)
	switch kind {
	case "", "Deployment", "deployment":
		dep, err := c.Client.AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return err
		}
		if dep.Spec.Template.Annotations == nil {
			dep.Spec.Template.Annotations = map[string]string{}
		}
		dep.Spec.Template.Annotations["kubectl.kubernetes.io/restartedAt"] = stamp
		_, err = c.Client.AppsV1().Deployments(namespace).Update(ctx, dep, metav1.UpdateOptions{})
		return err
	case "StatefulSet", "statefulset":
		s, err := c.Client.AppsV1().StatefulSets(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return err
		}
		if s.Spec.Template.Annotations == nil {
			s.Spec.Template.Annotations = map[string]string{}
		}
		s.Spec.Template.Annotations["kubectl.kubernetes.io/restartedAt"] = stamp
		_, err = c.Client.AppsV1().StatefulSets(namespace).Update(ctx, s, metav1.UpdateOptions{})
		return err
	case "DaemonSet", "daemonset":
		d, err := c.Client.AppsV1().DaemonSets(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return err
		}
		if d.Spec.Template.Annotations == nil {
			d.Spec.Template.Annotations = map[string]string{}
		}
		d.Spec.Template.Annotations["kubectl.kubernetes.io/restartedAt"] = stamp
		_, err = c.Client.AppsV1().DaemonSets(namespace).Update(ctx, d, metav1.UpdateOptions{})
		return err
	default:
		return fmt.Errorf("rollout não suportado para %s", kind)
	}
}

func (c *Cluster) Undo(ctx context.Context, namespace, name string) error {
	if err := c.ready(); err != nil {
		return err
	}
	dep, err := c.Client.AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return err
	}
	list, err := c.Client.AppsV1().ReplicaSets(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return err
	}
	var prev *appsv1.ReplicaSet
	var prevRev int
	curRev := revisionOf(dep.Annotations)
	for i := range list.Items {
		rs := &list.Items[i]
		if !ownedBy(rs.OwnerReferences, string(dep.UID)) {
			continue
		}
		rev := revisionOf(rs.Annotations)
		if rev > 0 && rev < curRev && (prev == nil || rev > prevRev) {
			prev = rs
			prevRev = rev
		}
	}
	if prev == nil {
		return fmt.Errorf("não há revisão anterior de %s", name)
	}
	dep.Spec.Template = prev.Spec.Template
	_, err = c.Client.AppsV1().Deployments(namespace).Update(ctx, dep, metav1.UpdateOptions{})
	return err
}

func (c *Cluster) Events(ctx context.Context, namespace string) ([]map[string]string, error) {
	if err := c.ready(); err != nil {
		return nil, err
	}
	list, err := c.Client.CoreV1().Events(nsOrAll(namespace)).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	out := make([]map[string]string, 0, len(list.Items))
	for _, e := range list.Items {
		out = append(out, map[string]string{
			"namespace": e.Namespace, "name": e.InvolvedObject.Name, "reason": e.Reason,
			"message": e.Message, "type": e.Type,
		})
	}
	return out, nil
}

func (c *Cluster) WhatBroke(ctx context.Context, namespace string) ([]Problem, error) {
	if err := c.ready(); err != nil {
		return nil, err
	}
	list, err := c.Client.CoreV1().Pods(nsOrAll(namespace)).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	return Problems(list.Items), nil
}

func (c *Cluster) ready() error {
	if c == nil || c.Client == nil {
		return fmt.Errorf("cluster Kubernetes indisponível")
	}
	return nil
}

func nsOrAll(ns string) string {
	if ns == "" || ns == "*" {
		return metav1.NamespaceAll
	}
	return ns
}

func replicas(n *int32) int32 {
	if n == nil {
		return 1
	}
	return *n
}

func images(spec corev1.PodSpec) string {
	if len(spec.Containers) == 0 {
		return ""
	}
	return spec.Containers[0].Image
}

func workload(kind, ns, name string, replicas, ready int32, image string) WorkloadView {
	return WorkloadView{Kind: kind, Namespace: ns, Name: name, Replicas: replicas, Ready: ready, Image: image}
}

func revisionOf(ann map[string]string) int {
	if ann == nil {
		return 0
	}
	var n int
	fmt.Sscanf(ann["deployment.kubernetes.io/revision"], "%d", &n)
	return n
}

func retryConflict(fn func() error) error {
	var err error
	for i := 0; i < 6; i++ {
		err = fn()
		if err == nil || !apierrors.IsConflict(err) {
			return err
		}
		time.Sleep(time.Duration(i+1) * 200 * time.Millisecond)
	}
	return err
}

func ownedBy(refs []metav1.OwnerReference, uid string) bool {
	for _, r := range refs {
		if string(r.UID) == uid {
			return true
		}
	}
	return false
}
