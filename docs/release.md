# macOS releases

eipgam ships through the [Homebrew tap](https://github.com/sichengchen/homebrew-tap).
The [Prepare macOS release workflow](../.github/workflows/cut-release-macos.yml)
builds and tests both architectures, combines them into a universal app, and
hands the ad hoc signed bundle and its source commit to the shared signing repo.
Apple credentials are managed by the [signing environment](https://github.com/sichengchen/macos-release#configure-once).
The release GitHub App needs access to eipgam and the tap.

1. In [macos-release Actions](https://github.com/sichengchen/macos-release/actions/workflows/release-macos.yml),
   run **Sign and notarize macOS release** on `main` with `app=eipgam`.
   Choose `current` for the first release at 0.2.0, or choose a bump from
   the highest stable source tag. Leave the run ID blank to start a build.
   You can also supply the run ID of a successful **Prepare macOS release** run.
   When the build and CI may take more than 50 minutes, start that workflow
   separately and pass its completed run ID.
2. Leave **Publish** off for the signing check. For a release, enable
   **Publish**. The signer verifies the app, staples the notarization ticket,
   tags the built source commit `vX.Y.Z`, and uploads
   `eipgam-X.Y.Z-universal.zip` under `eipgam-vX.Y.Z` in the signing repo.
3. The shared Homebrew job writes `Casks/eipgam.rb` with the checksum of
   the published ZIP. Check the tap before announcing the version. If the
   tap job fails after publication, rerun the failed job. If a source build
   needs a retry, start a fresh preparation run so both native artifacts
   belong to the same attempt.
