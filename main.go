package main

import (
	"fmt"
	"github.com/intel-go/cpuid"
	"log"
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

func generateLdPreload(existingLdPreload string) string {
	lib := fmt.Sprintf("LD_PRELOAD=%v/libtensorflow_cpu_%v.so", tensorflow_root, getLibtensorflowCpuSuffix())
	if existingLdPreload != "" {
		lib += fmt.Sprintf(":%v", existingLdPreload)
	}
	return lib
}

func main() {

	if len(os.Args) < 2 {
		os.Stderr.WriteString(fmt.Sprintf("Usage: %v <command> [args...]\n", os.Args[0]))
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
			log.Fatalf("exec.LookPath() failed: %v", err)
		}
	}

	args := os.Args[1:]

	os.Stderr.WriteString(fmt.Sprintf("Executing with %v\n", ldPreload))

	err := syscall.Exec(binary, args, env)
	if err != nil {
		log.Fatalf("syscall.Exec() failed: %v", err)
	}
}
