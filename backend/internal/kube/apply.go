package kube

import (
	"bytes"
	"context"
	"fmt"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/yaml"
)

// ApplyYAML cria ou atualiza os objetos conhecidos do manifesto.
func (c *Cluster) ApplyYAML(ctx context.Context, raw []byte, defaultNS string) ([]string, error) {
	if err := c.ready(); err != nil {
		return nil, err
	}
	if defaultNS == "" {
		defaultNS = "default"
	}
	decode := scheme.Codecs.UniversalDeserializer()
	var applied []string
	for _, doc := range bytes.Split(raw, []byte("\n---")) {
		doc = bytes.TrimSpace(doc)
		if len(doc) == 0 || bytes.HasPrefix(doc, []byte("#")) {
			continue
		}
		jsonDoc, err := yaml.YAMLToJSON(doc)
		if err != nil {
			return applied, err
		}
		if len(bytes.TrimSpace(jsonDoc)) == 0 || bytes.Equal(bytes.TrimSpace(jsonDoc), []byte("null")) {
			continue
		}
		obj, _, err := decode.Decode(jsonDoc, nil, nil)
		if err != nil {
			return applied, fmt.Errorf("yaml: %w", err)
		}
		name, err := c.applyObject(ctx, obj, defaultNS)
		if err != nil {
			return applied, err
		}
		applied = append(applied, name)
	}
	if applied == nil {
		return nil, fmt.Errorf("nenhum objeto no yaml")
	}
	return applied, nil
}

func (c *Cluster) applyObject(ctx context.Context, obj runtime.Object, defaultNS string) (string, error) {
	switch o := obj.(type) {
	case *corev1.Namespace:
		return c.upsertNS(ctx, o)
	case *corev1.ConfigMap:
		if o.Namespace == "" {
			o.Namespace = defaultNS
		}
		_, err := c.Client.CoreV1().ConfigMaps(o.Namespace).Get(ctx, o.Name, metav1.GetOptions{})
		if apierrors.IsNotFound(err) {
			_, err = c.Client.CoreV1().ConfigMaps(o.Namespace).Create(ctx, o, metav1.CreateOptions{})
			return "ConfigMap/" + o.Namespace + "/" + o.Name, err
		}
		if err != nil {
			return "", err
		}
		_, err = c.Client.CoreV1().ConfigMaps(o.Namespace).Update(ctx, o, metav1.UpdateOptions{})
		return "ConfigMap/" + o.Namespace + "/" + o.Name, err
	case *corev1.Service:
		if o.Namespace == "" {
			o.Namespace = defaultNS
		}
		cur, err := c.Client.CoreV1().Services(o.Namespace).Get(ctx, o.Name, metav1.GetOptions{})
		if apierrors.IsNotFound(err) {
			_, err = c.Client.CoreV1().Services(o.Namespace).Create(ctx, o, metav1.CreateOptions{})
			return "Service/" + o.Name, err
		}
		if err != nil {
			return "", err
		}
		o.ResourceVersion = cur.ResourceVersion
		o.Spec.ClusterIP = cur.Spec.ClusterIP
		_, err = c.Client.CoreV1().Services(o.Namespace).Update(ctx, o, metav1.UpdateOptions{})
		return "Service/" + o.Name, err
	case *appsv1.Deployment:
		if o.Namespace == "" {
			o.Namespace = defaultNS
		}
		cur, err := c.Client.AppsV1().Deployments(o.Namespace).Get(ctx, o.Name, metav1.GetOptions{})
		if apierrors.IsNotFound(err) {
			_, err = c.Client.AppsV1().Deployments(o.Namespace).Create(ctx, o, metav1.CreateOptions{})
			return "Deployment/" + o.Namespace + "/" + o.Name, err
		}
		if err != nil {
			return "", err
		}
		o.ResourceVersion = cur.ResourceVersion
		_, err = c.Client.AppsV1().Deployments(o.Namespace).Update(ctx, o, metav1.UpdateOptions{})
		return "Deployment/" + o.Namespace + "/" + o.Name, err
	case *appsv1.StatefulSet:
		if o.Namespace == "" {
			o.Namespace = defaultNS
		}
		_, err := c.Client.AppsV1().StatefulSets(o.Namespace).Create(ctx, o, metav1.CreateOptions{})
		if apierrors.IsAlreadyExists(err) {
			cur, gerr := c.Client.AppsV1().StatefulSets(o.Namespace).Get(ctx, o.Name, metav1.GetOptions{})
			if gerr != nil {
				return "", gerr
			}
			o.ResourceVersion = cur.ResourceVersion
			_, err = c.Client.AppsV1().StatefulSets(o.Namespace).Update(ctx, o, metav1.UpdateOptions{})
		}
		return "StatefulSet/" + o.Name, err
	case *appsv1.DaemonSet:
		if o.Namespace == "" {
			o.Namespace = defaultNS
		}
		_, err := c.Client.AppsV1().DaemonSets(o.Namespace).Create(ctx, o, metav1.CreateOptions{})
		if apierrors.IsAlreadyExists(err) {
			cur, gerr := c.Client.AppsV1().DaemonSets(o.Namespace).Get(ctx, o.Name, metav1.GetOptions{})
			if gerr != nil {
				return "", gerr
			}
			o.ResourceVersion = cur.ResourceVersion
			_, err = c.Client.AppsV1().DaemonSets(o.Namespace).Update(ctx, o, metav1.UpdateOptions{})
		}
		return "DaemonSet/" + o.Name, err
	case *networkingv1.Ingress:
		if o.Namespace == "" {
			o.Namespace = defaultNS
		}
		_, err := c.Client.NetworkingV1().Ingresses(o.Namespace).Create(ctx, o, metav1.CreateOptions{})
		if apierrors.IsAlreadyExists(err) {
			cur, gerr := c.Client.NetworkingV1().Ingresses(o.Namespace).Get(ctx, o.Name, metav1.GetOptions{})
			if gerr != nil {
				return "", gerr
			}
			o.ResourceVersion = cur.ResourceVersion
			_, err = c.Client.NetworkingV1().Ingresses(o.Namespace).Update(ctx, o, metav1.UpdateOptions{})
		}
		return "Ingress/" + o.Name, err
	default:
		return "", fmt.Errorf("tipo %T ainda não entra no apply", obj)
	}
}

func (c *Cluster) upsertNS(ctx context.Context, o *corev1.Namespace) (string, error) {
	_, err := c.Client.CoreV1().Namespaces().Get(ctx, o.Name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		_, err = c.Client.CoreV1().Namespaces().Create(ctx, o, metav1.CreateOptions{})
		return "Namespace/" + o.Name, err
	}
	return "Namespace/" + o.Name, err
}

func (c *Cluster) EnsureNamespace(ctx context.Context, name string) error {
	if err := c.ready(); err != nil {
		return err
	}
	_, err := c.Client.CoreV1().Namespaces().Create(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: name}}, metav1.CreateOptions{})
	if apierrors.IsAlreadyExists(err) {
		return nil
	}
	return err
}

func (c *Cluster) DeleteNamespace(ctx context.Context, name string) error {
	if err := c.ready(); err != nil {
		return err
	}
	err := c.Client.CoreV1().Namespaces().Delete(ctx, name, metav1.DeleteOptions{})
	if apierrors.IsNotFound(err) {
		return nil
	}
	return err
}

func splitDocs(raw string) []string {
	return strings.Split(raw, "---")
}
