package main

import (
	"fmt"
	"github.com/anight/gpu-monitoring-tools/bindings/go/nvml"
	"os"
)

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
			fmt.Fprintf(os.Stderr, "Error getting device %d: %v\n", i, err)
			os.Exit(1)
		}

		ret = append(ret, device)

		fmt.Fprintf(os.Stderr, "GPU %v: Path: %v, Model: %v, UUID: %v, CudaComputeCapability: %v.%v\n",
			i, device.Path, *device.Model, device.UUID, device.CudaComputeCapability.Major, device.CudaComputeCapability.Minor)
	}

	if count == 0 {
		fmt.Fprintf(os.Stderr, "No nvidia gpu(s) detected\n")
	}

	return
}

func gpuInit() {
	gpuList = gpuListAll()
}
