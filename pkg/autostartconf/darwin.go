// Copyright (c) 2020 brev & AUTHORS.
// All rights reserved.

// Redistribution and use in source and binary forms, with or without
// modification, are permitted provided that the following conditions are met:

// 1. Redistributions of source code must retain the above copyright notice, this
//    list of conditions and the following disclaimer.

// 2. Redistributions in binary form must reproduce the above copyright notice,
//    this list of conditions and the following disclaimer in the documentation
//    and/or other materials provided with the distribution.

// 3. Neither the name of the copyright holder nor the names of its
//    contributors may be used to endorse or promote products derived from
//    this software without specific prior written permission.

// THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS "AS IS"
// AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE
// IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE ARE
// DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT HOLDER OR CONTRIBUTORS BE LIABLE
// FOR ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR CONSEQUENTIAL
// DAMAGES (INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR
// SERVICES; LOSS OF USE, DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER
// CAUSED AND ON ANY THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY,
// OR TORT (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE
// OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.

package autostartconf

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
)

func init() {
	_ = InstallSystemDaemonDarwin
	_ = UninstallSystemDaemonDarwin
}

// darwinLaunchdPlist is the launchd.plist that's written to
// /Library/LaunchDaemons/com.brev.brev.plist or (in the
// future) a user-specific location.
//
// See man launchd.plist.
const darwinLaunchdPlist = `
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>

  <key>Label</key>
  <string>com.brev.brev</string>

  <key>ProgramArguments</key>
  <array>
    <string>/usr/local/bin/brev</string>
  </array>

  <key>RunAtLoad</key>
  <true/>

</dict>
</plist>
`

const (
	sysPlist  = "/Library/LaunchDaemons/com.brev.brev.plist"
	targetBin = "/usr/local/bin/brev"
	service   = "com.brev.brev"
)

func UninstallSystemDaemonDarwin(args []string) (ret error) {
	if len(args) > 0 {
		return errors.New("uninstall subcommand takes no arguments")
	}

	plist, err := exec.Command("launchctl", "list", "com.brev.brev").Output()
	_ = plist // parse it? https://github.com/DHowett/go-plist if we need something.
	running := err == nil

	if running {
		out, err := exec.Command("launchctl", "stop", "com.brev.brev").CombinedOutput()
		if err != nil {
			fmt.Printf("launchctl stop com.brev.brev: %v, %s\n", err, out)
			ret = err
		}
		out, err = exec.Command("launchctl", "unload", sysPlist).CombinedOutput()
		if err != nil {
			fmt.Printf("launchctl unload %s: %v, %s\n", sysPlist, err, out)
			if ret == nil {
				ret = err
			}
		}
	}

	if err := os.Remove(sysPlist); err != nil {
		if os.IsNotExist(err) {
			err = nil
		}
		if ret == nil {
			ret = err
		}
	}
	if err := os.Remove(targetBin); err != nil {
		if os.IsNotExist(err) {
			err = nil
		}
		if ret == nil {
			ret = err
		}
	}
	return ret
}

func InstallSystemDaemonDarwin(args []string) (err error) {
	if len(args) > 0 {
		return errors.New("install subcommand takes no arguments")
	}
	defer func() {
		if err != nil && os.Getuid() != 0 { // todo this does not work
			err = fmt.Errorf("%w; try running brev with sudo", err)
		}
	}()

	// Best effort:
	UninstallSystemDaemonDarwin(nil)

	// TODO probably does not work on m1 mac
	// Copy ourselves to /usr/local/bin/brev.
	if err := os.MkdirAll(filepath.Dir(targetBin), 0755); err != nil {
		return err
	}
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to find our own executable path: %w", err)
	}
	tmpBin := targetBin + ".tmp"
	f, err := os.Create(tmpBin)
	if err != nil {
		return err
	}
	self, err := os.Open(exe)
	if err != nil {
		f.Close()
		return err
	}
	_, err = io.Copy(f, self)
	self.Close()
	if err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmpBin, 0755); err != nil {
		return err
	}
	if err := os.Rename(tmpBin, targetBin); err != nil {
		return err
	}

	if err := ioutil.WriteFile(sysPlist, []byte(darwinLaunchdPlist), 0700); err != nil {
		return err
	}

	if out, err := exec.Command("launchctl", "load", sysPlist).CombinedOutput(); err != nil {
		return fmt.Errorf("error running launchctl load %s: %v, %s", sysPlist, err, out)
	}

	if out, err := exec.Command("launchctl", "start", service).CombinedOutput(); err != nil {
		return fmt.Errorf("error running launchctl start %s: %v, %s", service, err, out)
	}

	return nil
}
