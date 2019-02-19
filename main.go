package main

import (
	"fmt"
	"os"
	"os/exec"
	"path"
	"strings"
	"syscall"
)

var tensorflow_root = "/local/tensorflow/lib"

func getLibtensorflowCpuSuffix() string {
	return currentCpu().vectorFeatureSetBase
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

func generateLdPreload(existingLdPreload string) string {
	var lib string
	if len(gpuList) > 0 && haveLibtensorflowGpuSo() {
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

	cpuInit()

	gpuInit()

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
