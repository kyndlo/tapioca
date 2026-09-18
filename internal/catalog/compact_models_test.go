package catalog

import "testing"

func TestPinnedCompactModels(t *testing.T) {
	for _, tc := range []struct {
		ref, repo, file, revision, checksum string
		bytes                               int64
	}{
		{"minicpm5-2b", "openbmb/MiniCPM5-2B-GGUF", "MiniCPM5-2B-Q4_K_M.gguf", "2079a22f3beaa4e306449978533478fe0522f4b3", "ec2d5801640099e97d8d7e8003ad4d81f336e757811f03a26173dddf386602fd", 1561318368},
		{"spark-x2.5-4b", "XHToken/Spark-X2.5-4B-GGUF", "Spark-X2.5-4B-Q4_K_M.gguf", "902d865994943ab9235670e24f01846ee06091f2", "adfcfa19a4ed6a5985da8bf565fe15f8e1a7e131d79bae2d19d48d1c40109428", 2600224352},
	} {
		t.Run(tc.ref, func(t *testing.T) {
			resolved, err := ResolveForPlatform(tc.ref, "darwin", "arm64")
			if err != nil {
				t.Fatal(err)
			}
			if resolved.Kind != "" || resolved.Repo != tc.repo || resolved.Filename != tc.file ||
				resolved.SizeBytes != tc.bytes || resolved.SHA256 != tc.checksum ||
				resolved.URL != "https://huggingface.co/"+tc.repo+"/resolve/"+tc.revision+"/"+tc.file ||
				DefaultContext(tc.ref) != 8192 {
				t.Fatalf("unexpected compact model: %#v", resolved)
			}
		})
	}
}
