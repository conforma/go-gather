package k8s_p

import (
	"context"
	"fmt"
	"strings"

	"github.com/enterprise-contract/go-gather/metadata"
	"github.com/enterprise-contract/go-gather/metadata/k8s_p"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

type K8SPGatherer struct{}

// k8s*::configmap/ns/name#somekey
func (f *K8SPGatherer) Gather(ctx context.Context, sourceRaw, destination string) (metadata.Metadata, error) {

	source, err := parseSource(sourceRaw)
	if err != nil {
		return nil, fmt.Errorf("parsing source: %w", err)
	}

	remoteRef, err := resolveReference(ctx, source)
	if err != nil {
		return nil, fmt.Errorf("resolving reference: %s", err)
	}

	return k8s_p.K8SPMetadata{Ref: remoteRef}, nil
}

type sourceRef struct {
	kind      string
	namespace string
	name      string
	extra     string
}

func parseSource(source string) (*sourceRef, error) {
	parts := strings.Split(strings.TrimPrefix(source, "k8s*::"), "/")
	if len(parts) != 3 {
		return nil, fmt.Errorf("source must have three parts, <kind>/<ns>/<name>, got: %s", source)
	}
	s := sourceRef{
		kind:      strings.ToLower(parts[0]),
		namespace: parts[1],
	}
	s.name, s.extra, _ = strings.Cut(parts[2], "#")
	return &s, nil
}

func resolveReference(ctx context.Context, source *sourceRef) (string, error) {
	client, err := newClient()
	if err != nil {
		return "", fmt.Errorf("creating new client: %w", err)
	}

	switch source.kind {
	case "configmap":
		if source.extra == "" {
			return "", fmt.Errorf("source reference must contain configmap key")
		}
		cm, err := client.CoreV1().ConfigMaps(source.namespace).Get(ctx, source.name, metav1.GetOptions{})
		if err != nil {
			return "", fmt.Errorf("fetching configmap: %w", err)
		}
		remoteRef, found := cm.Data[source.extra]
		if !found {
			return "", fmt.Errorf("configmap data key %q not found", source.extra)
		}
		remoteRef = strings.TrimSpace(remoteRef)
		if remoteRef == "" {
			return "", fmt.Errorf("remote ref is empty")
		}
		return remoteRef, nil
	}
	return "", fmt.Errorf("%q is not supported", source.kind)
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
