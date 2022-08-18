package functional

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	ginkgo "github.com/onsi/ginkgo/v2"
	k8sv1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"kubevirt.io/client-go/kubecli"
	"kubevirt.io/kubevirt/pkg/util/cluster"
	kubevirttest "kubevirt.io/kubevirt/tests"
	kubevirttestutils "kubevirt.io/kubevirt/tests/util"
)

var KubeVirtStorageClassLocal string

//GetJobTypeEnvVar returns "JOB_TYPE" enviroment varibale
func GetJobTypeEnvVar() string {
	return (os.Getenv("JOB_TYPE"))
}

func ForwardPortsFromService(service *k8sv1.Service, ports []string, stop chan struct{}, readyTimeout time.Duration) error {
	selector := labels.FormatLabels(service.Spec.Selector)

	targetPorts := []string{}
	for _, p := range ports {
		split := strings.Split(p, ":")
		if len(split) != 2 {
			return fmt.Errorf("invalid port mapping for %s", p)
		}
		found := false
		for _, servicePort := range service.Spec.Ports {
			if split[1] == strconv.Itoa(int(servicePort.Port)) {
				targetPorts = append(targetPorts, split[0]+":"+servicePort.TargetPort.String())
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("port %s not found on service", split[1])
		}
	}
	cli, err := kubecli.GetKubevirtClient()
	if err != nil {
		return err
	}

	pods, err := cli.CoreV1().Pods(service.Namespace).List(context.Background(), v1.ListOptions{
		LabelSelector: selector,
	})
	if err != nil {
		return err
	}

	var targetPod *k8sv1.Pod
ForLoop:
	for _, pod := range pods.Items {
		if pod.Status.Phase != k8sv1.PodRunning {
			continue
		}
		for _, conditions := range pod.Status.Conditions {
			if conditions.Type == k8sv1.PodReady && conditions.Status == k8sv1.ConditionTrue {
				targetPod = &pod
				break ForLoop
			}
		}
	}

	if targetPod == nil {
		return fmt.Errorf("no ready pod listening on the service")
	}

	return kubevirttest.ForwardPorts(targetPod, targetPorts, stop, readyTimeout)
}

func IsOpenShift() bool {
	virtClient, err := kubecli.GetKubevirtClient()
	kubevirttestutils.PanicOnError(err)

	isOpenShift, err := cluster.IsOnOpenShift(virtClient)
	if err != nil {
		fmt.Printf("ERROR: Can not determine cluster type %v\n", err)
		panic(err)
	}

	return isOpenShift
}

func SkipIfNotOpenShift(message string) {
	if !IsOpenShift() {
		ginkgo.Skip("Not running on openshift: " + message)
	}
}
