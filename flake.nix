{
  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixpkgs-unstable";
    flake-parts.url = "github:hercules-ci/flake-parts";
    devenv.url = "github:cachix/devenv";
  };

  outputs = inputs@{ flake-parts, ... }:
    flake-parts.lib.mkFlake { inherit inputs; } {
      imports = [ inputs.devenv.flakeModule ];
      systems = inputs.nixpkgs.lib.systems.flakeExposed;
      perSystem = { config, lib, pkgs, ... }:
        let
          cfg = config.devenv.shells.default.languages.go;
          goVersion = (lib.versions.major cfg.package.version)
            + (lib.versions.minor cfg.package.version);
          buildWithSpecificGo = pkg:
            pkg.override {
              buildGoModule =
                pkgs."buildGo${goVersion}Module".override { go = cfg.package; };
            };
        in {
          devenv.shells.default = {
            languages.go.enable = true;
            languages.go.package = pkgs.go_1_23;
            packages = [
              (buildWithSpecificGo pkgs.golangci-lint)
              (buildWithSpecificGo pkgs.gofumpt)
            ];
          };
        };
    };
}
