package catalog

func graniteModel() Model {
	return Model{Name: "granite-4.2", Context: 8192, Repo: "ibm-granite/granite-4.2-3b-GGUF", Default: "3b-q4_k_m",
		Files: map[string]string{"3b-q4_k_m": "granite-4.2-3b-Q4_K_M.gguf"},
		Sizes: map[string]string{"3b-q4_k_m": "2.09 GiB"}, Memory: map[string]string{"3b-q4_k_m": "8 GiB min; 16 GiB recommended at 8K context"},
		GPUs:     map[string]string{"3b-q4_k_m": "CPU, Metal, Vulkan"},
		Features: map[string]string{"3b-q4_k_m": "Text chat; embedded thinking template; tool calling and FIM not qualified"},
		License:  "Apache-2.0", LicenseURL: "https://huggingface.co/ibm-granite/granite-4.2-3b",
		Downloads: map[string]Download{"3b-q4_k_m": {Revision: "47a3d9699d7539606c83943d717fcea7bd9f6a19", SizeBytes: 2244012160, SHA256: "20e436143017578687f7f848225cc6c6038126c84149192229c7dff6e4e0f427"}},
	}
}
