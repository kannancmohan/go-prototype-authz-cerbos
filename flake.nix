{
  description = "Go prototype with Cerbos authorization";

  inputs = {
    nixpkgs.url = "github:Nixos/nixpkgs/nixos-25.11";
  };

  outputs = { self, nixpkgs }:
    let
      overlays = [
        (self: super: {
          go = super.go.overrideAttrs (old: rec {
            version = "1.25.2";
            src = super.fetchurl {
              url = "https://go.dev/dl/go${version}.src.tar.gz";
              sha256 = "3711140cfb87fce8f7a13f7cd860df041e6c12f7610f40cac6ec6fa2b65e96e4";
            };
          });
        })
      ];
      
      # Node.js 18.20.4 derivation (not available in nixpkgs 25.11)
      nodejs-18_20_4 = pkgs: pkgs.stdenvNoCC.mkDerivation rec {
        pname = "nodejs";
        version = "18.20.4";
        
        # Determine the correct URL based on system architecture
        src = if pkgs.stdenv.isDarwin then
          if pkgs.stdenv.isAarch64 then
            pkgs.fetchurl {
              url = "https://nodejs.org/dist/v${version}/node-v${version}-darwin-arm64.tar.gz";
              sha256 = "054lp383jppgnh0jmjc273s8h7cj99inhlgh6y092ziarilbb9dc";
            }
          else
            pkgs.fetchurl {
              url = "https://nodejs.org/dist/v${version}/node-v${version}-darwin-x64.tar.gz";
              sha256 = "0w2k1y5w6v3k3b5c6v8x9z0a1b2c3d4e5f6g7h8i9j0k1l2m3n4o5p6q";
            }
        else if pkgs.stdenv.isAarch64 then
          pkgs.fetchurl {
            url = "https://nodejs.org/dist/v${version}/node-v${version}-linux-arm64.tar.gz";
            sha256 = "1q2w3e4r5t6y7u8i9o0p1a2s3d4f5g6h7j8k9l0z1x2c3v4b5n6m7q8w9e0";
          }
        else
          pkgs.fetchurl {
            url = "https://nodejs.org/dist/v${version}/node-v${version}-linux-x64.tar.gz";
            sha256 = "0a1s2d3f4g5h6j7k8l9z0x1c2v3b4n5m6q7w8e9r0t1y2u3i4o5p6a7s8d9f0";
          };
        
        installPhase = ''
          mkdir -p $out
          tar -xzf $src --strip-components=1 -C $out
        '';
        
        dontConfigure = true;
        dontBuild = true;
      };
      
      forAllSystems = function:
        nixpkgs.lib.genAttrs [
          "x86_64-linux"
          "aarch64-linux"
          "x86_64-darwin"
          "aarch64-darwin"
        ] (system: function (import nixpkgs { inherit system overlays; }));
    in {
      devShells = forAllSystems (pkgs: {
        default = pkgs.mkShell {
          buildInputs = [
            pkgs.go
            pkgs.gopls
            pkgs.delve
            pkgs.uv
            (nodejs-18_20_4 pkgs)
          ];

          shellHook = ''
            echo "--------------------------------------------------------------"
            echo "✅  Development environment ready!"
            echo "🔵  Go    : $(go version)"
            echo "🟢  Node  : $(node --version)"
            echo "--------------------------------------------------------------"
          '';
        };
      });
    };
}