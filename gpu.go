package main

import (
	"fmt"
	"github.com/anight/gpu-monitoring-tools/bindings/go/nvml"
	"os"
	"strconv"
	"strings"
)

type gpuComputeCapabilityType nvml.CudaComputeCapabilityInfo

var gpuList []*nvml.Device

func gpuListAll() (ret []*nvml.Device) {
	if err := nvml.Init(); err != nil {
		fmt.Fprintf(os.Stderr, "nvml.Init() failed: %v\n", err)
		return
	}
	defer nvml.Shutdown()

	count, err := nvml.GetDeviceCount()
	if err != nil {
		fmt.Fprintf(os.Stderr, "nvml.GetDeviceCount() failed: %v\n", err)
		return
	}

	driverVersion, err := nvml.GetDriverVersion()
	if err != nil {
		fmt.Fprintf(os.Stderr, "nvml.GetDriverVersion() failed: %v\n", err)
		return
	}

	fmt.Fprintf(os.Stderr, "Nvidia driver version: %v\n", driverVersion)

	for i := uint(0); i < count; i++ {
		device, err := nvml.NewDevice(i)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error getting device GPU%d: %v\n", i, err)
			os.Exit(1)
		}

		ret = append(ret, device)

		fmt.Fprintf(os.Stderr, "GPU%d: Path: %s, Model: %s, UUID: %s, CudaComputeCapability: %d.%d\n",
			i, device.Path, *device.Model, device.UUID, device.CudaComputeCapability.Major, device.CudaComputeCapability.Minor)
	}

	if count == 0 {
		fmt.Fprintf(os.Stderr, "No nvidia gpu(s) detected\n")
	}

	return
}

func gpuCudaComputeCapabilityInfoListParse(input string) (ret []gpuComputeCapabilityType, err error) {

	if input == "" {
		return
	}

	for _, item := range strings.Split(input, ",") {
		majmin := strings.Split(item, ".")
		if len(majmin) != 2 {
			err = fmt.Errorf("can't parse gpu compute capability list item: %s", item)
			return
		}
		ccc := gpuComputeCapabilityType{}
		ccc.Major, err = strconv.Atoi(majmin[0])
		if err != nil {
			return
		}
		ccc.Minor, err = strconv.Atoi(majmin[1])
		if err != nil {
			return
		}
		ret = append(ret, ccc)
	}
	return
}

func gpuUnsupportedDevices(lst []gpuComputeCapabilityType) []string {
	supportedCapabilities := make(map[nvml.CudaComputeCapabilityInfo]bool)
	for _, item := range lst {
		supportedCapabilities[nvml.CudaComputeCapabilityInfo(item)] = true
	}
	unsupportedDevices := []string{}
	for i, device := range gpuList {
		if _, ok := supportedCapabilities[device.CudaComputeCapability]; !ok {
			unsupportedDevices = append(unsupportedDevices, fmt.Sprintf("GPU%d", i))
		}
	}
	return unsupportedDevices
}

func gpuWarnIfUnsupportedDevices(lst []gpuComputeCapabilityType) {
	unsupportedDevices := gpuUnsupportedDevices(lst)
	if 0 != len(unsupportedDevices) {
		fmt.Fprintf(os.Stderr, "Warning: following GPU devices are unsupported in the selected libtensorflow build: %v\n", unsupportedDevices)
	}
}

func gpuInit() {
	gpuList = gpuListAll()
}
