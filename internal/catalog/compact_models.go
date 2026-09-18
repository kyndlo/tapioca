package catalog

func miniCPM5Model() Model {
	return Model{
		Name: "minicpm5-2b", Context: 8192,
		Repo: "openbmb/MiniCPM5-2B-GGUF", Default: "q4_k_m",
		Files: map[string]string{"q4_k_m": "MiniCPM5-2B-Q4_K_M.gguf"},
		Downloads: map[string]Download{"q4_k_m": {
			Revision:  "2079a22f3beaa4e306449978533478fe0522f4b3",
			SHA256:    "ec2d5801640099e97d8d7e8003ad4d81f336e757811f03a26173dddf386602fd",
			SizeBytes: 1561318368,
		}},
		Sizes:     map[string]string{"q4_k_m": "~1.45 GiB"},
		Memory:    map[string]string{"q4_k_m": "8 GiB test target; 16 GiB recommended at 8K context"},
		GPUs:      map[string]string{"q4_k_m": "CPU or Apple Metal; Vulkan pending qualification"},
		Languages: map[string]string{"q4_k_m": "English and Chinese"},
		Features:  map[string]string{"q4_k_m": "basic text chat; tools and long context not yet qualified"},
		License:   "Apache-2.0", LicenseURL: "https://huggingface.co/openbmb/MiniCPM5-2B",
	}
}

func sparkX25Model() Model {
	return Model{
		Name: "spark-x2.5-4b", Context: 8192,
		Repo: "XHToken/Spark-X2.5-4B-GGUF", Default: "q4_k_m",
		Files: map[string]string{"q4_k_m": "Spark-X2.5-4B-Q4_K_M.gguf"},
		Downloads: map[string]Download{"q4_k_m": {
			Revision:  "902d865994943ab9235670e24f01846ee06091f2",
			SHA256:    "adfcfa19a4ed6a5985da8bf565fe15f8e1a7e131d79bae2d19d48d1c40109428",
			SizeBytes: 2600224352,
		}},
		Sizes:     map[string]string{"q4_k_m": "~2.42 GiB"},
		Memory:    map[string]string{"q4_k_m": "8 GiB test target; 16 GiB recommended at 8K context"},
		GPUs:      map[string]string{"q4_k_m": "CPU or Apple Metal; Vulkan pending qualification"},
		Languages: map[string]string{"q4_k_m": "multilingual"},
		Features:  map[string]string{"q4_k_m": "basic text chat; tools and long context not yet qualified"},
		License:   "Apache-2.0", LicenseURL: "https://huggingface.co/XHToken/Spark-X2.5-4B",
	}
}
