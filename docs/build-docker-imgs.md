# Building docker images

The `fissile build` command comes with a helpful subcommand for building docker images from a **BOSH** release.

The command is `fissile build release-images`. See the [docs](/generated/fissile_build_release-images.md)

## How to use it?

The following are the flags that you need to provide, in order to generate assets and building the Docker image.

- `--stemcell`

    You need to provide a reference to a docker image, containing the stemcell you want to use, as the underlying layer. For example:

  ```sh
  --stemcell=splatform/fissile-stemcell-opensuse:42.3-36.g03b4653-30.80
  ```

  **Note**: You docker stemcell image is required to have two specific labels. They are `stemcell-flavor` and `stemcell-version`.

- `--name`

    This is the name of the BOSH release. This should be the value defined in the BOSH [deployment manifest](https://bosh.io/docs/manifest-v2/#releases).

- `--version`

    The version of the BOSH release. This should be the value defined in the BOSH [deployment manifest](https://bosh.io/docs/manifest-v2/#releases).

- `--sha1`
    The sha1 of the BOSH release. This should be the value defined in the BOSH [deployment manifest](https://bosh.io/docs/manifest-v2/#releases).

- `-url`
    The url of the BOSH release. This should be the value defined in the BOSH [deployment manifest](https://bosh.io/docs/manifest-v2/#releases).

Additionally you could also just generate assets, without forcing `fissile` to generate a new Docker image. For this, use the `--no-build` flag.
