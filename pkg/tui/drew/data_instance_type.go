package drew

type InstanceType struct {
	Cloud    Cloud
	GPUModel string
	GPUCount int
	VRAM     string
	CPUModel string
	CPUCount int
	Memory   string
	Storage  string
}

var (
	Crusoe_1x_a100_40gb InstanceType = InstanceType{
		Cloud:    CloudCrusoe,
		GPUModel: "NVIDIA A100",
		GPUCount: 1,
		VRAM:     "40GB",
		CPUModel: "Intel Xeon (Ice Lake)",
		CPUCount: 12,
		Memory:   "120GB",
		Storage:  "1x960GB",
	}
	Crusoe_2x_a100_40gb InstanceType = InstanceType{
		Cloud:    CloudCrusoe,
		GPUModel: "NVIDIA A100",
		GPUCount: 2,
		VRAM:     "40GB",
		CPUModel: "Intel Xeon (Ice Lake)",
		CPUCount: 24,
		Memory:   "240GB",
		Storage:  "2x960GB",
	}
	Crusoe_4x_a100_40gb InstanceType = InstanceType{
		Cloud:    CloudCrusoe,
		GPUModel: "NVIDIA A100",
		GPUCount: 4,
		VRAM:     "40GB",
		CPUModel: "Intel Xeon (Ice Lake)",
		CPUCount: 48,
		Memory:   "480GB",
		Storage:  "4x960GB",
	}
	Crusoe_8x_a100_40gb InstanceType = InstanceType{
		GPUModel: "NVIDIA A100",
		GPUCount: 8,
		VRAM:     "40GB",
		CPUModel: "Intel Xeon (Ice Lake)",
		CPUCount: 96,
		Memory:   "960GB",
		Storage:  "8x960GB",
	}

	Lambda_1x_a100_40gb InstanceType = InstanceType{
		Cloud:    CloudLambda,
		GPUModel: "NVIDIA A100",
		GPUCount: 1,
		VRAM:     "40GB",
		CPUModel: "Intel Xeon (Sapphire Rapids)",
		CPUCount: 30,
		Memory:   "225GiB",
		Storage:  "512GiB",
	}
	Lambda_2x_a100_40gb InstanceType = InstanceType{
		Cloud:    CloudLambda,
		GPUModel: "NVIDIA A100",
		GPUCount: 2,
		VRAM:     "40GB",
		CPUModel: "Intel Xeon (Sapphire Rapids)",
		CPUCount: 60,
		Memory:   "450GiB",
		Storage:  "1TiB",
	}
	Lambda_4x_a100_40gb InstanceType = InstanceType{
		Cloud:    CloudLambda,
		GPUModel: "NVIDIA A100",
		GPUCount: 4,
		VRAM:     "40GB",
		CPUModel: "Intel Xeon (Sapphire Rapids)",
		CPUCount: 120,
		Memory:   "900GiB",
		Storage:  "1TiB",
	}
)
