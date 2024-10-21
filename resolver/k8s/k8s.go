package k8s

import (
	"context"
	"fmt"
	"strings"

	"github.com/enterprise-contract/go-gather/gather"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

type configMapKeyResolver struct{}

func (r configMapKeyResolver) Resolve(ctx context.Context, rawRef string) (string, error) {
	ref, err := newConfigMapKeyRef(rawRef)
	if err != nil {
		return "", fmt.Errorf("parsing configmap ref %q: %w", rawRef, err)
	}

	client, err := newClient()
	if err != nil {
		return "", fmt.Errorf("creating new client: %w", err)
	}

	return resolveReference(ctx, client, ref)
}

type configMapKeyRef struct {
	namespace string
	name      string
	key       string
}

func newConfigMapKeyRef(source string) (*configMapKeyRef, error) {
	parts := strings.Split(source, "/")
	if len(parts) != 3 {
		return nil, fmt.Errorf("configmap ref must have three parts, <ns>/<name>/key>, got: %s", source)
	}
	return &configMapKeyRef{
		namespace: parts[0],
		name:      parts[1],
		key:       parts[2],
	}, nil
}

func resolveReference(ctx context.Context, client *kubernetes.Clientset, source *configMapKeyRef) (string, error) {
	cm, err := client.CoreV1().ConfigMaps(source.namespace).Get(ctx, source.name, metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("fetching configmap: %w", err)
	}
	remoteRef, found := cm.Data[source.key]
	if !found {
		return "", fmt.Errorf("configmap data key %q not found", source.key)
	}
	remoteRef = strings.TrimSpace(remoteRef)
	if remoteRef == "" {
		return "", fmt.Errorf("remote ref is empty")
	}
	return remoteRef, nil
}

func newClient() (*kubernetes.Clientset, error) {
	rules := clientcmd.NewDefaultClientConfigLoadingRules()
	clientConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(rules, nil)

	config, err := clientConfig.ClientConfig()
	if err != nil {
		return nil, err
	}

	return kubernetes.NewForConfig(config)
}

func init() {
	gather.RegisterResolver("k8sConfigMapKey", configMapKeyResolver{})
}
