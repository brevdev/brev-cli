package setup

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func Test_genSetupHunkForLanguage(t *testing.T) {
	tests := map[string]struct {
		lang    string
		version string
		want    string
	}{
		"go basic": {
			lang: "go", version: "1.16.9", want: `
wget https://golang.org/dl/go1.16.9.linux-amd64.tar.gz -O go.tar.gz
sudo rm -rf /usr/local/go && sudo tar -C /usr/local -xzf go.tar.gz
echo "" | sudo tee -a ~/.bashrc
echo "export PATH=\$PATH:/usr/local/go/bin" | sudo tee -a ~/.bashrc
source ~/.bashrc
echo "" | sudo tee -a ~/.zshrc
echo "export PATH=\$PATH:/usr/local/go/bin" | sudo tee -a ~/.zshrc
source ~/.zshrc
rm go.tar.gz
`,
		},
		"lang doesn't exist": {
			lang: "lakfjdfj", version: "1.16.9", want: defaultSetupScript,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			got, err := genSetupHunkForLanguage(tc.lang, tc.version)
			if err != nil {
				panic(err)
			}
			diff := cmp.Diff(tc.want, got)
			if diff != "" {
				t.Fatalf(diff)
			}
		})
	}
}
