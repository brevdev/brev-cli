class BrevCli < Formula
  desc "CLI tool for managing workspaces provided by brev.dev"
  homepage "https://docs.brev.dev"
  url "https://github.com/brevdev/brev-cli/archive/refs/tags/v0.6.12.tar.gz"
  sha256 "5237a3706e88f76e9a4d97109272f491539ad45ff50fc3fdb12fd478c55c0774"
  license "MIT"
  depends_on "go" => :build

  def install
    ldflags = "-X github.com/brevdev/brev-cli/pkg/cmd/version.Version=v#{version}"
    system "go", "build", *std_go_args(output: bin/"brev", ldflags: ldflags)
    bin.install "brev"
  end

  test do
    system "#{bin}/brev", " --version"
  end
end
