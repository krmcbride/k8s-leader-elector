{
  description = "Compatible replacement for k8s.gcr.io/leader-elector:0.5";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs =
    {
      self,
      nixpkgs,
      flake-utils,
    }:
    flake-utils.lib.eachDefaultSystem (
      system:
      let
        pkgs = import nixpkgs { inherit system; };
        buildGoModule = pkgs.buildGoModule.override { go = pkgs.go_1_25; };
        leaderElector = buildGoModule {
          pname = "k8s-leader-elector";
          version = "0.1.0";
          src = self;
          vendorHash = "sha256-+r2jsGfk4LC+farORyyhhMMIO3359JdASCglCL/PEqo=";
          ldflags = [
            "-s"
            "-w"
            "-X main.version=0.1.0"
            "-X main.revision=${self.shortRev or self.dirtyShortRev or "unknown"}"
          ];
          subPackages = [ "cmd/leader-elector" ];
        };
      in
      {
        packages = {
          default = leaderElector;
          image = pkgs.dockerTools.buildLayeredImage {
            name = "k8s-leader-elector";
            tag = self.shortRev or self.dirtyShortRev or "dev";
            contents = [
              pkgs.cacert
              leaderElector
            ];
            config = {
              Entrypoint = [ "${leaderElector}/bin/leader-elector" ];
              ExposedPorts = {
                "4040/tcp" = { };
              };
              User = "65532:65532";
            };
          };
        };

        checks = {
          go-test = leaderElector.overrideAttrs (_: {
            name = "k8s-leader-elector-go-test";
            doCheck = true;
            checkPhase = ''
              runHook preCheck
              go test ./...
              go vet ./...
              runHook postCheck
            '';
            installPhase = ''
              mkdir -p $out
              touch $out/check-passed
            '';
          });

          lint = leaderElector.overrideAttrs (old: {
            name = "k8s-leader-elector-lint";
            nativeBuildInputs = (old.nativeBuildInputs or [ ]) ++ [ pkgs.golangci-lint ];
            doCheck = true;
            buildPhase = ''
              runHook preBuild
              runHook postBuild
            '';
            checkPhase = ''
              runHook preCheck
              export GOLANGCI_LINT_CACHE="$TMPDIR/golangci-lint-cache"
              golangci-lint run ./...
              runHook postCheck
            '';
            installPhase = ''
              mkdir -p $out
              touch $out/lint-passed
            '';
          });
        };

        devShells.default = pkgs.mkShell {
          packages = [
            pkgs.go_1_25
            pkgs.gopls
            pkgs.gotools
            pkgs.golangci-lint
            pkgs.go-containerregistry
            pkgs.kubectl
            pkgs.ko
            pkgs.docker-client
          ];
        };
      }
    );
}
