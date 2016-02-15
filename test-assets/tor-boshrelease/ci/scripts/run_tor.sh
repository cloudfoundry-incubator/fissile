#!/bin/bash

# change to root of bosh release
DIR=$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )
cd $DIR/../..

cat > ~/.bosh_config << EOF
---
aliases:
  target:
    bosh-lite: ${bosh_target}
auth:
  ${bosh_target}:
    username: ${bosh_username}
    password: ${bosh_password}
EOF
bosh target ${bosh_target}

mkdir -p tmp
hostname_yml=tmp/hostname.yml
cat > ${hostname_yml} << YAML
jobs:
  tor:
    instances: 1
    properties:
      tor:
        hostname: 37rdhjniwhxqgpt7.onion
        private_key: |
          -----BEGIN RSA PRIVATE KEY-----
          MIICXQIBAAKBgQDIoaKPSL1u20pa10ZePtZbeZ5rNI92dlZegppMWEGxGMnXGHv+
          8detyIjko+oBDReFlHmrClVLFl+g75gyyjwbS/j/GDSSfvKttoSl6Yn9M5UMXplX
          ys0IrCHtMsiCRTx+ybyNBxW16fI2dnGJ8ZFNQ43roy+BphOIva0v9j4uLwIDAQAB
          AoGARM9S3ouXFMc3GDLPGpG4mQT8NU6AiaOKeb2XR+nZFfEngJMQK98sFpk5ghlJ
          r3SbBaBnnibcG/WfdKXX8Et2E1ciycNVHio76A4owWDaYEyrr11yoViDSXyaWQob
          6XzPRiQz5477ekdI+yZYVBIFcISJ8oYMB/Pye3nh6HWFZQkCQQDlN46cFkswZh3B
          shgFmijW3jc/JtrB4Hsa0l7OFDlHGHr7eP8hmzR9tWUfdiau6yfnKxaigWkAIzo0
          TtGL87wTAkEA4BMC9nnmmGLIWyLDifxpwbRQpqatVfRyJKVs1b6qXTMSfh2y1WVx
          jHSNd2zMgYkbsQblW6ExK3xv+MaLQB4Q9QJBAInXPiBpW6/wSMa5ha6gxRxpp4mH
          oRfkGcPIbJC7IrK5awOdALhB8HAETJp938dizK08gTEaZ31YseDQ4TyrRxcCQE5e
          N9YOcljvi5VcRjlXX4GQ1/hBKTR7xwQMG1FyWtE30IrtRiOeVCVEikmvcqMHWfkD
          KWpvqOvFnL/MaN1m5pUCQQCxaHB4FR+HaWJaWhbxdUDnRhK3usRY4bwIPsDXxGL8
          SCQuzg3HtWOLOlB06N2jWcT+JajUb4pLiAuunD+D105T
          -----END RSA PRIVATE KEY-----
YAML

_bosh() {
  bosh -n $@
}

set -e

./templates/make_manifest warden ${hostname_yml}

_bosh deploy
