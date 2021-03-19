package allocation

import (
	"flag"
	"fmt"
	"log"
	"path/filepath"

	agonesv1 "agones.dev/agones/pkg/apis/agones/v1"
	allocationv1 "agones.dev/agones/pkg/apis/allocation/v1"
	"agones.dev/agones/pkg/client/clientset/versioned"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

func connectToAgones() (*versioned.Clientset, error) {
	var kubeconfig *string
	if home := homedir.HomeDir(); home != "" {
		kubeconfig = flag.String("kubeconfig", filepath.Join(home, ".kube", "config"), "(optional) absolute path to the kubeconfig file")
	} else {
		kubeconfig = flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
	}
	flag.Parse()

	// use the current context in kubeconfig
	config, err := clientcmd.BuildConfigFromFlags("", *kubeconfig)
	if err != nil {
		return nil, err
	}

	agonesClient, err := versioned.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	return agonesClient, nil
}

func createAgonesGameServerAllocation() *allocationv1.GameServerAllocation {
	return &allocationv1.GameServerAllocation{
		Spec: allocationv1.GameServerAllocationSpec{
			Required: metav1.LabelSelector{
				MatchLabels: map[string]string{agonesv1.FleetNameLabel: "medieval-game-server-fleet"},
			},
		},
	}
}

func AllocateGameServer() (*allocationv1.GameServerAllocation, error) {

	agonesClient, err := connectToAgones()
	if err != nil {
		return nil, err
	}

	gsa, err := agonesClient.AllocationV1().GameServerAllocations("medieval-game-server").Create(createAgonesGameServerAllocation())
	if err != nil {
		log.Printf("error requesting allocation: %v\n", err)
		return nil, err
	}

	if gsa.Status.State != allocationv1.GameServerAllocationAllocated {
		log.Printf("failed to allocate game server\n")
		return nil, fmt.Errorf("failed to allocate game server")
	}

	return gsa, nil
}
