{
  inputs.nixpkgs.url = "github:NixOS/nixpkgs/nixpkgs-unstable";
  inputs.flake-utils.url = "github:numtide/flake-utils";
  inputs.flake-compat = { url = "github:edolstra/flake-compat"; flake = false; };
  inputs.devshell.url = "github:numtide/devshell";
  inputs.devshell.inputs.nixpkgs.follows = "nixpkgs";

  outputs = { self, nixpkgs, flake-utils, ... }@inputs:
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = import nixpkgs {
          inherit system;
          overlays = [ inputs.devshell.overlays.default ];
          config.allowUnfreePredicate = pkg: builtins.elem (nixpkgs.lib.getName pkg) [
            "claude-code"
          ];
        };
        selfPkgs = self.packages.${system};

        # packages
        go = pkgs.go_1_25;
      in
      {
        devShell = pkgs.devshell.mkShell {
          devshell = {
            name = "sandbox-probe";
            motd = "";
            packages = with pkgs; [
              go
              gcc
              buf

              nono
              gemini-cli
              claude-code
            ];
          };
        };
      }
    );
}
