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

	// 4. 获取vcuda占用的设备
	k8sclient, hostname, err := GetClientAndHostName()
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
	fmt.Printf("in ues device %v", devUsage)
	return devUsage

}

func IsMig(index int) bool {
	handle, ret := nvml2.DeviceGetHandleByIndex(index)
	if ret != nvml2.SUCCESS {
		fmt.Println("DeviceGetHandleByIndex err, index: ", index)
	}
	isMig, _ := handle.IsMigDeviceHandle()
	return isMig
}

func GetNvidiaDevice(client kubernetes.Interface, hostname string) ([]string, error) {
	allPods, err := getPodsOnNode(client, hostname, string(v1.PodRunning))
	if err != nil {
		return nil, err
	}
	//gpuModKey := fmt.Sprintf("inspur.com/gpu-mod-idx-%d", containerId)
	//gpuIdxKey := fmt.Sprintf("inspur.com/gpu-index-idx-%d", containerId)
	//gpuPciKey := fmt.Sprintf("inspur.com/gpu-gpuPcieId-idx-%d", containerId)

	devMap := make(map[string]struct{}, 0)
	for _, pod := range allPods {
		for i, _ := range pod.Spec.Containers {
			if idxStr, ok := pod.ObjectMeta.Annotations[fmt.Sprintf("inspur.com/gpu-index-idx-%d", i)]; ok {
				idxList := strings.Split(idxStr, "-")
				for _, idx := range idxList {
					if _, err := strconv.Atoi(idx); err != nil {
						return nil, fmt.Errorf("predicate idx %s invalid for pod %s ", idxStr, pod.UID)
					}
					devStr := NvidiaDevicePrefix + idxStr
					if !IsValidGPUPath(devStr) {
						return nil, fmt.Errorf("predicate idx %s invalid", devStr)
					}
					if _, ok := devMap[idxStr]; !ok {
						devMap[idxStr] = struct{}{}
					}
				}
			}
		}
	}
	devList := []string{}
	for dev, _ := range devMap {
		devList = append(devList, dev)
	}
	return devList, nil
}
func getPodsOnNode(client kubernetes.Interface, hostname string, phase string) ([]v1.Pod, error) {
	if len(hostname) == 0 {
		hostname, _ = os.Hostname()
	}
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
		if err != nil {
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
	return k8sclient, hostname, nil

}
