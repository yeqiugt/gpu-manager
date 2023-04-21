package nvidia

import (
	"fmt"
	nvml2 "github.com/NVIDIA/go-nvml/pkg/nvml"
	"strconv"
)

func GetAnnotation(containerId int, deviceIds []string) map[string]string {
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
	var (
		gpuIdx    string
		gpuPcieId string
		gpuMod    string
	)

	gpuModKey := fmt.Sprintf("inspur.com/gpu-mod-idx-%d", containerId)
	gpuIdxKey := fmt.Sprintf("inspur.com/gpu-index-idx-%d", containerId)
	gpuPciKey := fmt.Sprintf("inspur.com/gpu-gpuPcieId-idx-%d", containerId)

	for _, deviceId := range deviceIds {
		fmt.Println("222222222222222222request ids : ", deviceId)
		index, _ := strconv.Atoi(deviceId)
		handle, ret := nvml2.DeviceGetHandleByIndex(index)
		if ret != nvml2.SUCCESS {
			fmt.Println("DeviceGetHandleByIndex err, index: ", index)
		}
		uuid, _ := handle.GetUUID()
		fmt.Println("333333333333333333request uuid : ", uuid)

		// handle gpu
		nvmlDevice, _ := nvml2.DeviceGetHandleByUUID(uuid)
		pcieInfo, _ := nvml2.DeviceGetPciInfo(nvmlDevice)
		/*			fmt.Printf("555555555555555555555gpu index: %s, gpu uuid : %s \n", reqDevice.Index, reqDevice.GetUUID())

					fmt.Printf("555555555555555555555gpu index: %s, gpu uuid : %s \n", reqDevice.Index, reqDevice.GetUUID())
					fmt.Printf("6666666666666666666pcie info : %+v \n", pcieInfo)
					fmt.Printf("PciDeviceId : %x\n", pcieInfo.PciDeviceId)
					fmt.Printf("Device : %x\n", pcieInfo.Device)
					fmt.Printf("Bus : %x\n", pcieInfo.Bus)
					fmt.Printf("BusId : %x\n", pcieInfo.BusId)
					fmt.Printf("BusIdLegacy : %x\n", pcieInfo.BusIdLegacy)
					fmt.Printf("PciSubSystemId : %x\n", pcieInfo.PciSubSystemId)*/

		if gpuPcieId == "" {
			gpuPcieId = fmt.Sprintf("%02x:%02x", pcieInfo.Bus, pcieInfo.Device)
		} else {
			gpuPcieId += "-" + fmt.Sprintf("%02x:%02x", pcieInfo.Bus, pcieInfo.Device)
		}
		if gpuIdx == "" {
			gpuIdx = fmt.Sprintf("%d", index)
		} else {
			gpuIdx += "-" + fmt.Sprintf("%d", index)
		}

		gpuMod = "vcuda"
		fmt.Println("777777777777777777777")
	}

	fmt.Println("888888888888888888888888")
	return map[string]string{
		gpuModKey: gpuMod,
		gpuIdxKey: gpuIdx,
		gpuPciKey: gpuPcieId,
	}
}
