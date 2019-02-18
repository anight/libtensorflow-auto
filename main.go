package main

import (
	"fmt"
	"github.com/anight/gpu-monitoring-tools/bindings/go/nvml"
	"github.com/intel-go/cpuid"
	"os"
	"os/exec"
	"path"
	"strings"
	"syscall"
)

var tensorflow_root = "/local/tensorflow/lib"

func getLibtensorflowCpuSuffix() string {
	if cpuid.EnabledAVX512 {
		/*
		   From man gcc-6:

		   skylake-avx512
		       Intel Skylake Server CPU with 64-bit extensions, MOVBE, MMX, SSE, SSE2, SSE3, SSSE3, SSE4.1, SSE4.2, POPCNT, PKU, AVX, AVX2, AES, PCLMUL,
		       FSGSBASE, RDRND, FMA, BMI, BMI2, F16C, RDSEED, ADCX, PREFETCHW, CLFLUSHOPT, XSAVEC, XSAVES, AVX512F, AVX512VL, AVX512BW, AVX512DQ and AVX512CD
		       instruction set support.
		*/
		if cpuid.HasExtendedFeature(cpuid.AVX512F) &&
			cpuid.HasExtendedFeature(cpuid.AVX512VL) &&
			cpuid.HasExtendedFeature(cpuid.AVX512BW) &&
			cpuid.HasExtendedFeature(cpuid.AVX512DQ) &&
			cpuid.HasExtendedFeature(cpuid.AVX512CD) {

			return "avx512"
		}
	}

	if cpuid.EnabledAVX {
		if cpuid.HasExtendedFeature(cpuid.AVX2) {
			return "avx2_fma"
		}
		if cpuid.HasFeature(cpuid.AVX) {
			return "avx"
		}
	}

	if cpuid.HasFeature(cpuid.SSE4_2) {
		return "sse42"
	}

	panic("what a funny cpu you have")
}

func haveLibtensorflowGpuSo() bool {
	so := fmt.Sprintf("%v/libtensorflow_gpu.so", tensorflow_root)
	_, err := os.Stat(so)
	if err == nil {
		return true
	} else {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		return false
	}
}

func listGPUs() (ret []*nvml.Device) {
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

func generateLdPreload(existingLdPreload string) string {
	var lib string
	if len(listGPUs()) > 0 && haveLibtensorflowGpuSo() {
		// XXX: With libtensorflow_gpu.so we don't do cpu features matching for now
		// XXX: With libtensorflow_gpu.so we don't do gpu cuda capabilities matching for now
		lib = fmt.Sprintf("LD_PRELOAD=%v/libtensorflow_gpu.so", tensorflow_root)
	} else {
		lib = fmt.Sprintf("LD_PRELOAD=%v/libtensorflow_cpu_%v.so", tensorflow_root, getLibtensorflowCpuSuffix())
	}
	if existingLdPreload != "" {
		lib += fmt.Sprintf(":%v", existingLdPreload)
	}
	return lib
}

func main() {

	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: %v <command> [args...]\n", os.Args[0])
		return
	}

	env := os.Environ()

	var ldPreload string
	foundLdPreload := false

	for i := range env {
		if strings.HasPrefix(env[i], "LD_PRELOAD=") {
			ldPreload = generateLdPreload(env[i][len("LD_PRELOAD="):])
			env[i] = ldPreload
			foundLdPreload = true
			break
		}
	}

	if !foundLdPreload {
		ldPreload = generateLdPreload("")
		env = append(env, ldPreload)
	}

	binary := os.Args[1]

	if path.Base(binary) == binary {
		var err error
		binary, err = exec.LookPath(binary)
		if err != nil {
			fmt.Fprintf(os.Stderr, "exec.LookPath() failed: %v", err)
			os.Exit(1)
		}
	}

	args := os.Args[1:]

	fmt.Fprintf(os.Stderr, "Setting %v\n", ldPreload)

	fmt.Fprintf(os.Stderr, "Executing %v\n", args)

	err := syscall.Exec(binary, args, env)
	if err != nil {
		fmt.Fprintf(os.Stderr, "syscall.Exec() failed: %v", err)
		os.Exit(1)
	}
}
