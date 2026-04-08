{
  lib,
  buildGo125Module,
  version,
  ...
}:
let
  pname = "brev";
in
buildGo125Module {
  inherit version pname;
  src = ./.;

  env.CGO_ENABLED = 1;
  hardeningDisable = [ "bindnow" ];
  subPackages = [ "." ];

  ldflags = [
    "-s"
    "-w"
    "-X github.com/brevdev/brev-cli/pkg/cmd/version.Version=${version}"
  ];

  vendorHash = "sha256-rB6uqkpnc+SlbzNvtTOnDCIJIpxoiyPb/lsiRYkDltg=";

  postInstall = ''
    mv $out/bin/brev-cli $out/bin/brev
  '';

  meta = {
    description = "CLI tool for managing workspaces provided by brev.nvidia.com";
    homepage = "https://docs.nvidia.com/brev/latest/";
    license = lib.licenses.mit;
    mainProgram = pname;
  };
}
