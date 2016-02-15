#!/bin/bash

set -e

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

cat > config/private.yml << EOF
---
blobstore:
  s3:
    access_key_id: ${aws_access_key_id}
    secret_access_key: ${aws_secret_access_key}
EOF

_bosh() {
  bosh -n $@
}

set -e

version=$(cat ../version/number)
if [ -z "$version" ]; then
  echo "missing version number"
  exit 1
fi
if [[ "${release_name}X" == "X" ]]; then
  echo "missing \$release_name"
  exit 1
fi

echo Prepare github release information
set -x
mkdir -p tmp/release_info
cp ci/release_notes.md tmp/release_info/notes.md
echo "${release_name} v${version}" > tmp/release_info/name
echo "v${version}" > tmp/release_info/tag
cat > tmp/release_info/slack_success_message.txt <<EOS
<!here> New version v${version} released
EOS

git config --global user.email ${ci_git_email}
git config --global user.name "CI Bot"

git merge --no-edit ${promotion_branch}

bosh target ${BOSH_TARGET}

bosh -n create release --final --with-tarball --version "$version"

git add -A
git commit -m "release v${version}"
