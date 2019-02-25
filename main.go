package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"regexp"
	"sort"
	"strings"
	"syscall"
)

var libtensorflowRoot = "/local/tensorflow/lib"

type libtensorflowSoType struct {
	name                               string
	minRequiredCpu                     cpu
	supportedCudaComputeCapabilityInfo []gpuComputeCapabilityType
}

func (ls libtensorflowSoType) CpuPriority() int {
	// return how many features in current cpu are not supported by ls
	unsupportedFeatures := currentCpu().allFeaturesCount() - ls.minRequiredCpu.allFeaturesCount()
	//	fmt.Fprintf(os.Stderr, "CpuPriority(%v): %v\n", ls, unsupportedFeatures)
	return unsupportedFeatures
}

func (ls libtensorflowSoType) GpuPriority() int {
	// return how many nvidia adapters are not supported by ls
	unsupportedDevices := gpuUnsupportedDevices(ls.supportedCudaComputeCapabilityInfo)
	//	fmt.Fprintf(os.Stderr, "GpuPriority(%v): %v\n", ls, unsupportedDevices)
	return len(unsupportedDevices)
}

func (ls libtensorflowSoType) warnFeaturesUnsupported() {
	ls.minRequiredCpu.cpuWarnIfUnsupportedTensorflowFeatures()
	gpuWarnIfUnsupportedDevices(ls.supportedCudaComputeCapabilityInfo)
}

func generateLdPreload(existingLdPreload string, libtensorflowSo libtensorflowSoType) string {
	lib := fmt.Sprintf("LD_PRELOAD=%s/%s", libtensorflowRoot, libtensorflowSo.name)
	if existingLdPreload != "" {
		lib += fmt.Sprintf(":%v", existingLdPreload)
	}
	return lib
}

func parseLibtensorflowSoFilename(filename string) (ret libtensorflowSoType, err error) {
	ret.name = filename
	pattern := regexp.MustCompile(`^libtensorflow(?:_gpu_(?P<gpu>[^_]+))?(?:_cpu_(?P<cpu>[^.]+))\.so$`)
	allIndexes := pattern.FindAllSubmatch([]byte(filename), 1)
	if len(allIndexes) != 1 {
		err = fmt.Errorf("can't parse file name")
		return
	}
	s := allIndexes[0]
	gpuCaptured := string(s[1])
	cpuCaptured := string(s[2])
	ret.minRequiredCpu, err = cpuParse(cpuCaptured)
	if err != nil {
		return
	}
	ret.supportedCudaComputeCapabilityInfo, err = gpuCudaComputeCapabilityInfoListParse(gpuCaptured)
	return
}

func scanLibtensorflowSo() (ret []libtensorflowSoType) {
	files, err := ioutil.ReadDir(libtensorflowRoot)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ioutil.ReadDir(\"%s\") failed: %v\n", libtensorflowRoot, err)
		os.Exit(1)
	}
	for _, f := range files {
		if f.IsDir() {
			continue
		}
		if 0 != (f.Mode() & os.ModeSymlink) {
			continue
		}
		libtensorflowSo, err := parseLibtensorflowSoFilename(f.Name())
		if err != nil {
			fmt.Fprintf(os.Stderr, "ignoring file %s: %v\n", f.Name(), err)
			continue
		}
		ret = append(ret, libtensorflowSo)
	}
	return
}

func selectLibtensorflowSo(lst []libtensorflowSoType) (libtensorflowSoType, error) {
	lstCompatible := []libtensorflowSoType{}
	for i := range lst {
		cpu := lst[i].minRequiredCpu
		if !cpu.allFeaturesSupported() {
			continue
		}
		lstCompatible = append(lstCompatible, lst[i])
	}

	if len(lstCompatible) == 0 {
		return libtensorflowSoType{}, fmt.Errorf("No compatible libtensorflow*.so found")
	}

	compareBetterCpu := func(i, j int) bool {
		i_ls, j_ls := lstCompatible[i], lstCompatible[j]
		return i_ls.CpuPriority() < j_ls.CpuPriority()
	}

	//	fmt.Fprintf(os.Stderr, "%v\n", lst)

	sort.SliceStable(lstCompatible, compareBetterCpu)

	//	fmt.Fprintf(os.Stderr, "%v\n", lstCompatible)

	compareBetterGpu := func(i, j int) bool {
		i_ls, j_ls := lstCompatible[i], lstCompatible[j]
		return i_ls.GpuPriority() < j_ls.GpuPriority()
	}

	sort.SliceStable(lstCompatible, compareBetterGpu)

	//	fmt.Fprintf(os.Stderr, "%v\n", lstCompatible)

	return lstCompatible[0], nil
}

func main() {

	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: %v <command> [args...]\n", os.Args[0])
		return
	}

	cpuInit()
	gpuInit()

	lst := scanLibtensorflowSo()

	if len(lst) == 0 {
		fmt.Fprintf(os.Stderr, "No libtensorflow*.so found in %s\n", libtensorflowRoot)
		os.Exit(1)
	}

	libtensorflowSo, err := selectLibtensorflowSo(lst)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}

	env := os.Environ()

	var ldPreload string
	foundLdPreload := false

	for i := range env {
		if strings.HasPrefix(env[i], "LD_PRELOAD=") {
			ldPreload = generateLdPreload(env[i][len("LD_PRELOAD="):], libtensorflowSo)
			env[i] = ldPreload
			foundLdPreload = true
			break
		}
	}

	if !foundLdPreload {
		ldPreload = generateLdPreload("", libtensorflowSo)
		env = append(env, ldPreload)
	}

	env = append(env, "CUDA_DEVICE_ORDER=PCI_BUS_ID")

	binary := os.Args[1]

	if path.Base(binary) == binary {
		var err error
		binary, err = exec.LookPath(binary)
		if err != nil {
			fmt.Fprintf(os.Stderr, "exec.LookPath() failed: %v\n", err)
			os.Exit(1)
		}
	}

	args := os.Args[1:]

	fmt.Fprintf(os.Stderr, "Setting %v\n", ldPreload)

	libtensorflowSo.warnFeaturesUnsupported()

	fmt.Fprintf(os.Stderr, "Executing %v\n", args)

	err = syscall.Exec(binary, args, env)
	if err != nil {
		fmt.Fprintf(os.Stderr, "syscall.Exec() failed: %v\n", err)
		os.Exit(1)
	}
}
