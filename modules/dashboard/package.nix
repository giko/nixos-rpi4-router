# Pre-built dashboard binary. The binary is produced by
# `.github/workflows/build-dashboard.yml`, published as a GitHub Release
# asset, and pinned here via `version.json`. CI auto-commits bumps to
# `version.json` after each successful release.
{ stdenvNoCC
, fetchurl
, lib
}:

let
  v = builtins.fromJSON (builtins.readFile ./version.json);
in
stdenvNoCC.mkDerivation {
  pname = "router-dashboard";
  version = v.version;

  # The placeholder "bootstrap" version evaluates lazily — it is never
  # realized because the NixOS module's mkIf guard prevents the package
  # from being used until a real version lands.
  src =
    if v.version == "bootstrap"
    then throw "router-dashboard is in bootstrap state; CI has not published a release yet"
    else fetchurl { url = v.url; hash = v.hash; };

  dontUnpack = true;

  installPhase = ''
    runHook preInstall
    install -Dm755 $src $out/bin/dashboard
    runHook postInstall
  '';

  meta = with lib; {
    description = "Read-only web dashboard for the nixos-rpi4-router (see modules/dashboard/docs/spec.md)";
    platforms = [ "aarch64-linux" ];
    license = licenses.mit;
    maintainers = [ ];
  };
}
