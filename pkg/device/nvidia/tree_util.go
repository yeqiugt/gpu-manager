/*
 * Tencent is pleased to support the open source community by making TKEStack available.
 *
 * Copyright (C) 2012-2019 Tencent. All Rights Reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License"); you may not use
 * this file except in compliance with the License. You may obtain a copy of the
 * License at
 *
 * https://opensource.org/licenses/Apache-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS, WITHOUT
 * WARRANTIES OF ANY KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations under the License.
 */

package nvidia

import (
	"fmt"
	nvml2 "github.com/NVIDIA/go-nvml/pkg/nvml"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/klog"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"tkestack.io/nvml"
)

func parseToGpuTopologyLevel(str string) nvml.GpuTopologyLevel {
	switch str {
	case "PIX":
		return nvml.TOPOLOGY_SINGLE
	case "PXB":
		return nvml.TOPOLOGY_MULTIPLE
	case "PHB":
		return nvml.TOPOLOGY_HOSTBRIDGE
	case "SOC":
		return nvml.TOPOLOGY_CPU
	}

	if strings.HasPrefix(str, "GPU") {
		return nvml.TOPOLOGY_INTERNAL
	}

	return nvml.TOPOLOGY_UNKNOWN
}

const (
	NvidiaCtlDevice    = "/dev/nvidiactl"
	NvidiaUVMDevice    = "/dev/nvidia-uvm"
	NvidiaFullpathRE   = `^/dev/nvidia([0-9]*)$`
	NvidiaDevicePrefix = "/dev/nvidia"
)

func GetInUseDevice() map[int]bool {

	// 4. 获取nvidia占用的设备
	k8sclient, hostname, err := GetClientAndHostName()
	if err != nil {
		fmt.Println("GetClientAndHostName err", err)
	}
	inUsedDev, err := GetNvidiaDevice(k8sclient, hostname)
	if err != nil {
		fmt.Println("GetNvidiaDevice err", err)
	}
	fmt.Println(" GetNvidiaDevice in use device", inUsedDev)

	devUsage := make(map[int]bool)
	for _, dev := range inUsedDev {
		index, err := strconv.Atoi(dev)
		if err != nil {
			fmt.Println(err)
		}
		devUsage[index] = true
	}
	fmt.Printf("in ues device %v \n", devUsage)

	return devUsage

}

func IsMig(index int) bool {
	fmt.Println("determined is mig, gpu index: ", index)
	ret := nvml2.Init()
	if ret != nvml2.SUCCESS {
		fmt.Println("nvlib init err")
	}
	defer func() {
		ret := nvml2.Shutdown()
		if ret != nvml2.SUCCESS {
			fmt.Println("Error shutting down NVML: %v", ret)
		}
	}()

	handle, ret := nvml2.DeviceGetHandleByIndex(index)
	if ret != nvml2.SUCCESS {
		fmt.Println("DeviceGetHandleByIndex err, index: ", index, ret)
	}
	currentMode, PendingMode, ret := handle.GetMigMode()
	fmt.Println("currentMode: ", currentMode, " PendingMode: ", PendingMode)
	if ret != nvml2.SUCCESS {
		fmt.Println("DeviceGetHandleByIndex err, index: ", index, ret)
	}
	if currentMode == nvml2.DEVICE_MIG_ENABLE {
		fmt.Println("gpu index", index, " is mig ", true)
		return true
	}
	fmt.Println("gpu index", index, " is mig ", false)
	return false
}

func GetNvidiaDevice(client kubernetes.Interface, hostname string) ([]string, error) {
	ret := nvml2.Init()
	if ret != nvml2.SUCCESS {
		fmt.Println("nvlib init err")
	}
	defer func() {
		ret := nvml2.Shutdown()
		if ret != nvml2.SUCCESS {
			fmt.Println("Error shutting down NVML: %v", ret)
		}
	}()
	allPods, err := getPodsOnNode(client, hostname, string(v1.PodRunning))
	//fmt.Println("all  pods :")
	//for _, pod := range allPods {
	//	fmt.Println(pod.Name)
	//}
	if err != nil {
		return nil, err
	}
	//gpuModKey := fmt.Sprintf("inspur.com/gpu-mod-idx-%d", containerId)
	//gpuIdxKey := fmt.Sprintf("inspur.com/gpu-index-idx-%d", containerId)
	//gpuPciKey := fmt.Sprintf("inspur.com/gpu-gpuPcieId-idx-%d", containerId)

	devMap := make(map[string]struct{}, 0)
	for _, pod := range allPods {
		for i, c := range pod.Spec.Containers {
			fmt.Println("pod name: ", pod.Name, "container name ", c.Name)

			annotation := fmt.Sprintf("inspur.com/gpu-index-idx-%d", i)
			fmt.Println("finding: ", annotation)
			gpuModKey := fmt.Sprintf("inspur.com/gpu-mod-idx-%d", i)

			if idxStr, ok := pod.ObjectMeta.Annotations[annotation]; ok {
				fmt.Println("1111111111111111111 found ", idxStr)
				if mod, ok := pod.ObjectMeta.Annotations[gpuModKey]; ok {
					idxList := strings.Split(idxStr, "-")
					modList := strings.Split(mod, "-")
					for i, idx := range idxList {
						if modList[i] == "vcuda" {
							continue
						}
						fmt.Println("found gpu idx : ", idx)
						if _, err := strconv.Atoi(idx); err != nil {
							return nil, fmt.Errorf("predicate idx %s invalid for pod %s ", idxStr, pod.UID)
						}
						fmt.Println("gpu index ", idx, " in use")
						devMap[idx] = struct{}{}
					}
				}

			}
		}
	}
	devList := []string{}
	for dev, _ := range devMap {
		devList = append(devList, dev)
	}
	fmt.Println("in use devcie List : ", devList)
	return devList, nil
}
func getPodsOnNode(client kubernetes.Interface, hostname string, phase string) ([]v1.Pod, error) {

	fmt.Println("hostname: ", hostname)
	fmt.Println("phase: ", phase)
	var (
		selector fields.Selector
		pods     []v1.Pod
	)

	if phase != "" {
		selector = fields.SelectorFromSet(fields.Set{"spec.nodeName": hostname, "status.phase": phase})
	} else {
		selector = fields.SelectorFromSet(fields.Set{"spec.nodeName": hostname})
	}
	var (
		podList *v1.PodList
		err     error
	)

	err = wait.PollImmediate(time.Second, time.Minute, func() (bool, error) {
		podList, err = client.CoreV1().Pods(v1.NamespaceAll).List(metav1.ListOptions{
			FieldSelector: selector.String(),
			LabelSelector: labels.Everything().String(),
		})
		fmt.Println("podList", podList)
		if err != nil {
			fmt.Println("get pod err: ", err)
			return false, err
		}
		return true, nil
	})
	if err != nil {
		return pods, fmt.Errorf("failed to get Pods on node %s because: %v", hostname, err)
	}

	klog.V(9).Infof("all pods on this node: %v", podList.Items)
	for _, pod := range podList.Items {
		pods = append(pods, pod)
	}

	return pods, nil
}

// IsValidGPUPath checks if path is valid Nvidia GPU device path
func IsValidGPUPath(path string) bool {
	return regexp.MustCompile(NvidiaFullpathRE).MatchString(path)
}

func GetClientAndHostName() (*kubernetes.Clientset, string, error) {
	// 1. 获取client
	config, err := rest.InClusterConfig()
	if err != nil {
		fmt.Println("get incluster config err")
		return &kubernetes.Clientset{}, "", err
	}
	k8sclient, err := kubernetes.NewForConfig(config)
	if err != nil {
		fmt.Println("getConfig err ", err)
		return &kubernetes.Clientset{}, "", err
	}
	hostname, _ := os.Hostname()
	gpuManagerPod, err := k8sclient.CoreV1().Pods("kube-system").Get(hostname, metav1.GetOptions{})
	if err != nil {
		fmt.Println("get gpumanager pod err: ", err)
		return nil, "", err
	}
	nodeName := gpuManagerPod.Spec.NodeName
	return k8sclient, nodeName, nil

}
