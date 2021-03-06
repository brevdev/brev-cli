package setupscript

import (
	"bytes"
	"html/template"
	"io"

	breverrors "github.com/brevdev/brev-cli/pkg/errors"
)

const DefaultSetupScript = `
#!/bin/bash

set -euo pipefail

####################################################################################
##### Specify software and dependencies that are required for this project     #####
#####                                                                          #####
##### Note:                                                                    #####
##### The working directory is /home/brev/<PROJECT_FOLDER_NAME>. Execution of  #####
##### this file happens at this level.                                         #####
####################################################################################

##### Yarn #####
# (echo ""; echo "##### Yarn #####"; echo "";)
# curl -sS https://dl.yarnpkg.com/debian/pubkey.gpg | sudo apt-key add
# echo "deb https://dl.yarnpkg.com/debian/ stable main" | sudo tee /etc/apt/sources.list.d/yarn.list
# sudo apt update
# sudo apt install -y yarn

##### Homebrew #####
# (echo ""; echo "##### Homebrew #####"; echo "";)
# curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh | bash -
# echo 'eval "$(/home/linuxbrew/.linuxbrew/bin/brew shellenv)"' >> /home/brev/.bash_profile
# echo 'eval "$(/home/linuxbrew/.linuxbrew/bin/brew shellenv)"' >> /home/brev/.zshrc
# eval "$(/home/linuxbrew/.linuxbrew/bin/brew shellenv)"

##### Node v14.x + npm #####
# (echo ""; echo "##### Node v14.x + npm #####"; echo "";)
# sudo apt install ca-certificates
# curl -fsSL https://deb.nodesource.com/setup_14.x | sudo -E bash -
# sudo apt-get install -y nodejs

# install npm packages globally without sudo
# modified from https://stackoverflow.com/questions/18088372/how-to-npm-install-global-not-as-root
# mkdir "${HOME}/.npm-packages"
# printf "prefix=${HOME}/.npm-packages" >> $HOME/.npmrc
# cat <<EOF | tee -a ~/.bashrc | tee -a ~/.zshrc
# NPM_PACKAGES="\${HOME}/.npm-packages"
# NODE_PATH="\${NPM_PACKAGES}/lib/node_modules:\${NODE_PATH}"
# PATH="\${NPM_PACKAGES}/bin:\${PATH}"
# # Unset manpath so we can inherit from /etc/manpath via the ` + "`manpath`\n" + `
# # command
# unset MANPATH # delete if you already modified MANPATH elsewhere in your config
# MANPATH="\${NPM_PACKAGES}/share/man:\$(manpath)"
# EOF

##### Python + Pip + Poetry #####
# (echo ""; echo "##### Python + Pip + Poetry #####"; echo "";)
# sudo apt-get install -y python3-distutils
# sudo apt-get install -y python3-apt
# curl -sSL https://raw.githubusercontent.com/python-poetry/poetry/master/get-poetry.py | python3 -
# curl https://bootstrap.pypa.io/get-pip.py -o get-pip.py
# python3 get-pip.py
# rm get-pip.py
# source $HOME/.poetry/env

##### Golang v16x #####
# (echo ""; echo "##### Golang v16x #####"; echo "";)
# wget https://golang.org/dl/go1.16.7.linux-amd64.tar.gz
# sudo rm -rf /usr/local/go && sudo tar -C /usr/local -xzf go1.16.7.linux-amd64.tar.gz
# echo "" | sudo tee -a ~/.bashrc
# echo "export PATH=\$PATH:/usr/local/go/bin" | sudo tee -a ~/.bashrc
# source ~/.bashrc
# echo "" | sudo tee -a ~/.zshrc
# echo "export PATH=\$PATH:/usr/local/go/bin" | sudo tee -a ~/.zshrc
# source ~/.zshrc
# rm go1.16.7.linux-amd64.tar.gz

##### Custom commands #####
# (echo ""; echo "##### Custom commands #####"; echo "";)
# npm install
`

type langHunk interface {
	SetVersion(string)
	WriteHunk(io.Writer) error
}

type GoHunk struct {
	Version string
}

func (gh *GoHunk) SetVersion(version string) {
	gh.Version = version
}

func (gh GoHunk) GetTemplateString() string {
	return `
wget https://golang.org/dl/go{{ .Version }}.linux-amd64.tar.gz -O go.tar.gz
sudo rm -rf /usr/local/go && sudo tar -C /usr/local -xzf go.tar.gz
echo "" | sudo tee -a ~/.bashrc
echo "export PATH=\$PATH:/usr/local/go/bin" | sudo tee -a ~/.bashrc
source ~/.bashrc
echo "" | sudo tee -a ~/.zshrc
echo "export PATH=\$PATH:/usr/local/go/bin" | sudo tee -a ~/.zshrc
source ~/.zshrc
rm go.tar.gz
`
}

func (gh GoHunk) GetTemplate() (*template.Template, error) {
	res, err := template.New("go").Parse(gh.GetTemplateString())
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	return res, nil
}

func (gh GoHunk) WriteHunk(w io.Writer) error {
	tmpl, err := gh.GetTemplate()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	err = tmpl.Execute(w, gh)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	return nil
}

func buildLangHunkMap() map[string]langHunk {
	langHunk := make(map[string]langHunk)
	langHunk["go"] = &GoHunk{}
	return langHunk
}

func GenSetupHunkForLanguage(language, version string) (string, error) {
	langHunkMap := buildLangHunkMap()
	lhWriter, ok := langHunkMap[language]
	if !ok {
		return DefaultSetupScript, nil
	}
	lhWriter.SetVersion(version)
	buf := new(bytes.Buffer)
	err := lhWriter.WriteHunk(buf)
	if err != nil {
		return "", breverrors.WrapAndTrace(err)
	}
	return buf.String(), nil
}
