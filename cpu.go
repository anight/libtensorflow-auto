package main

import (
	"fmt"
	"github.com/anight/cpuid"
	"math/bits"
	"os"
)

type cpu struct {
	gccName, alias   string
	features         cpuid.FeatureType
	extendedFeatures cpuid.ExtendedFeatureType
	extraFeatures    cpuid.ExtraFeatureType
}

type cpuArchitectureId int

const (
	cpuArchitectureUnknown cpuArchitectureId = iota
	cpuArchitectureNehalem
	cpuArchitectureWestmere
	cpuArchitectureSandybridge
	cpuArchitectureIvybridge
	cpuArchitectureHaswell
	cpuArchitectureBroadwell
	cpuArchitectureSkylake
	cpuArchitectureSkylakeAvx512
	cpuArchitectureCannonlake
	cpuArchitectureIcelakeClient
	cpuArchitectureIcelakeServer
)

var currentCpuArchitectureId cpuArchitectureId = cpuArchitectureUnknown
var allCPUs map[cpuArchitectureId]cpu

type cpuTensorflowFeatureType struct {
	feature string
	c       cpu
}

var cpuTensorflowFeatures = []cpuTensorflowFeatureType{
	{feature: "avx512", c: cpu{extendedFeatures: cpuid.AVX512F | cpuid.AVX512VL | cpuid.AVX512BW | cpuid.AVX512DQ | cpuid.AVX512CD}},
	{feature: "avx2", c: cpu{extendedFeatures: cpuid.AVX2}},
	{feature: "fma", c: cpu{features: cpuid.FMA}},
	{feature: "avx", c: cpu{features: cpuid.AVX}},
	{feature: "sse42", c: cpu{features: cpuid.SSE4_2}},
}

func currentCpu() cpu {
	return allCPUs[currentCpuArchitectureId]
}

func newCPU(basedOn, thisCpu cpu) cpu {
	thisCpu.features |= basedOn.features
	thisCpu.extendedFeatures |= basedOn.extendedFeatures
	thisCpu.extraFeatures |= basedOn.extraFeatures
	return thisCpu
}

func (c cpu) allFeaturesCount() int {
	return bits.OnesCount64(uint64(c.features)) + bits.OnesCount64(uint64(c.extendedFeatures)) + bits.OnesCount64(uint64(c.extraFeatures))
}

func (c cpu) allFeaturesSupported() bool {

	if !cpuid.EnabledAVX512 && 0 != (c.extendedFeatures&cpuid.AVX512F) {
		// processor support avx512 but currently running OS does not
		return false
	}
	if !cpuid.EnabledAVX && 0 != (c.features&cpuid.AVX) {
		// processor support avx but currently running OS does not
		return false
	}

	var bit uint64
	for bit = 1; bit != 0; bit <<= 1 {
		if 0 != (uint64(c.features) & bit) {
			if !cpuid.HasFeature(cpuid.FeatureType(bit)) {
				return false
			}
		}
		if 0 != (uint64(c.extendedFeatures) & bit) {
			if !cpuid.HasExtendedFeature(cpuid.ExtendedFeatureType(bit)) {
				return false
			}
		}
		if 0 != (uint64(c.extraFeatures) & bit) {
			if !cpuid.HasExtraFeature(cpuid.ExtraFeatureType(bit)) {
				return false
			}
		}
	}

	return true
}

func (c cpu) cpuTensorflowFeatures() (tensorflowFeatures []string) {
	for _, tf := range cpuTensorflowFeatures {
		if tf.c.features == (tf.c.features&c.features) &&
			tf.c.extendedFeatures == (tf.c.extendedFeatures&c.extendedFeatures) &&
			tf.c.extraFeatures == (tf.c.extraFeatures&c.extraFeatures) {
			tensorflowFeatures = append(tensorflowFeatures, tf.feature)
		}
	}
	return
}

func (c cpu) cpuWarnIfUnsupportedTensorflowFeatures() {
	cpuFeaturesUnsupported := cpu{
		features:         currentCpu().features &^ c.features,
		extendedFeatures: currentCpu().extendedFeatures &^ c.extendedFeatures,
		extraFeatures:    currentCpu().extraFeatures &^ c.extraFeatures,
	}

	unsupported := cpuFeaturesUnsupported.cpuTensorflowFeatures()

	if 0 != len(unsupported) {
		fmt.Fprintf(os.Stderr, "Warning: following CPU features are unsupported in the selected libtensorflow build, performance can be below optimal: %v\n", unsupported)
	}
}

func cpuParse(name string) (cpu, error) {
	for _, v := range allCPUs {
		if v.gccName == name || v.alias == name {
			return v, nil
		}
	}
	return cpu{}, fmt.Errorf("unknown cpu name: %s", name)
}

func cpuInit() {
	for k, v := range allCPUs {
		if !v.allFeaturesSupported() {
			continue
		}
		if allCPUs[currentCpuArchitectureId].allFeaturesCount() < v.allFeaturesCount() {
			currentCpuArchitectureId = k
		}
	}

	if currentCpuArchitectureId == cpuArchitectureUnknown {
		panic("unsupported cpu")
	}

	fmt.Fprintf(os.Stderr, "CPU: %s %v\n", currentCpu().gccName, currentCpu().cpuTensorflowFeatures())
}

func init() {

	allCPUs = make(map[cpuArchitectureId]cpu)

	// from man gcc-8:
	//
	// nehalem
	//     Intel Nehalem CPU with 64-bit extensions, MMX, SSE, SSE2, SSE3, SSSE3, SSE4.1, SSE4.2 and POPCNT instruction set support.
	allCPUs[cpuArchitectureNehalem] = newCPU(cpu{}, cpu{gccName: "nehalem", alias: "sse42", features: cpuid.SSE4_2})
	// westmere
	//     Intel Westmere CPU with 64-bit extensions, MMX, SSE, SSE2, SSE3, SSSE3, SSE4.1, SSE4.2, POPCNT, AES and PCLMUL instruction set support.
	allCPUs[cpuArchitectureWestmere] = newCPU(allCPUs[cpuArchitectureNehalem], cpu{gccName: "westmere", features: cpuid.AES})
	// sandybridge
	//     Intel Sandy Bridge CPU with 64-bit extensions, MMX, SSE, SSE2, SSE3, SSSE3, SSE4.1, SSE4.2, POPCNT, AVX, AES and PCLMUL instruction set support.
	allCPUs[cpuArchitectureSandybridge] = newCPU(allCPUs[cpuArchitectureWestmere], cpu{gccName: "sandybridge", alias: "avx", features: cpuid.AVX})
	// ivybridge
	//     Intel Ivy Bridge CPU with 64-bit extensions, MMX, SSE, SSE2, SSE3, SSSE3, SSE4.1, SSE4.2, POPCNT, AVX, AES, PCLMUL, FSGSBASE, RDRND and F16C
	//     instruction set support.
	allCPUs[cpuArchitectureIvybridge] = newCPU(allCPUs[cpuArchitectureSandybridge], cpu{gccName: "ivybridge", features: cpuid.F16C})
	// haswell
	//     Intel Haswell CPU with 64-bit extensions, MOVBE, MMX, SSE, SSE2, SSE3, SSSE3, SSE4.1, SSE4.2, POPCNT, AVX, AVX2, AES, PCLMUL, FSGSBASE, RDRND,
	//     FMA, BMI, BMI2 and F16C instruction set support.
	allCPUs[cpuArchitectureHaswell] = newCPU(allCPUs[cpuArchitectureIvybridge], cpu{gccName: "haswell", alias: "avx2_fma", extendedFeatures: cpuid.AVX2, features: cpuid.FMA})
	// broadwell
	//     Intel Broadwell CPU with 64-bit extensions, MOVBE, MMX, SSE, SSE2, SSE3, SSSE3, SSE4.1, SSE4.2, POPCNT, AVX, AVX2, AES, PCLMUL, FSGSBASE, RDRND,
	//     FMA, BMI, BMI2, F16C, RDSEED, ADCX and PREFETCHW instruction set support.
	allCPUs[cpuArchitectureBroadwell] = newCPU(allCPUs[cpuArchitectureHaswell], cpu{gccName: "broadwell", extraFeatures: cpuid.PREFETCHW})
	// skylake
	//     Intel Skylake CPU with 64-bit extensions, MOVBE, MMX, SSE, SSE2, SSE3, SSSE3, SSE4.1, SSE4.2, POPCNT, AVX, AVX2, AES, PCLMUL, FSGSBASE, RDRND,
	//     FMA, BMI, BMI2, F16C, RDSEED, ADCX, PREFETCHW, CLFLUSHOPT, XSAVEC and XSAVES instruction set support.
	allCPUs[cpuArchitectureSkylake] = newCPU(allCPUs[cpuArchitectureBroadwell], cpu{gccName: "skylake", extendedFeatures: cpuid.CLFLUSHOPT})
	// skylake-avx512
	//     Intel Skylake Server CPU with 64-bit extensions, MOVBE, MMX, SSE, SSE2, SSE3, SSSE3, SSE4.1, SSE4.2, POPCNT, PKU, AVX, AVX2, AES, PCLMUL,
	//     FSGSBASE, RDRND, FMA, BMI, BMI2, F16C, RDSEED, ADCX, PREFETCHW, CLFLUSHOPT, XSAVEC, XSAVES, AVX512F, CLWB, AVX512VL, AVX512BW, AVX512DQ and
	//     AVX512CD instruction set support.
	allCPUs[cpuArchitectureSkylakeAvx512] = newCPU(allCPUs[cpuArchitectureSkylake], cpu{gccName: "skylake-avx512", alias: "avx512", extendedFeatures: cpuid.AVX512F | cpuid.AVX512VL | cpuid.AVX512BW | cpuid.AVX512DQ | cpuid.AVX512CD})
	// cannonlake
	//     Intel Cannonlake Server CPU with 64-bit extensions, MOVBE, MMX, SSE, SSE2, SSE3, SSSE3, SSE4.1, SSE4.2, POPCNT, PKU, AVX, AVX2, AES, PCLMUL,
	//     FSGSBASE, RDRND, FMA, BMI, BMI2, F16C, RDSEED, ADCX, PREFETCHW, CLFLUSHOPT, XSAVEC, XSAVES, AVX512F, AVX512VL, AVX512BW, AVX512DQ, AVX512CD,
	//     AVX512VBMI, AVX512IFMA, SHA and UMIP instruction set support.
	allCPUs[cpuArchitectureCannonlake] = newCPU(allCPUs[cpuArchitectureSkylakeAvx512], cpu{gccName: "cannonlake", extendedFeatures: cpuid.SHA})
}
