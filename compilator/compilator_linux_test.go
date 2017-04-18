package compilator

import (
	"bytes"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/hpcloud/fissile/model"
	"github.com/hpcloud/termui"
	"github.com/stretchr/testify/assert"
)

type LockedBuffer struct {
	mutex sync.Mutex
	buf   bytes.Buffer
}

func (b *LockedBuffer) Write(p []byte) (n int, err error) {
	b.mutex.Lock()
	defer b.mutex.Unlock()
	return b.buf.Write(p)
}

func (b *LockedBuffer) String() string {
	b.mutex.Lock()
	defer b.mutex.Unlock()
	return b.buf.String()
}

func TestCompilePackageInChroot(t *testing.T) {
	assert := assert.New(t)

	if os.Geteuid() != 0 {
		t.Skip("building without docker requires root permissions")
	}

	stderr := &LockedBuffer{}
	ui := termui.New(&bytes.Buffer{}, stderr, nil)

	workDir, err := os.Getwd()
	assert.NoError(err)

	releasePath := filepath.Join(workDir, "../test-assets/no-license")
	releasePathBoshCache := filepath.Join(releasePath, "bosh-cache")
	release, err := model.NewDevRelease(releasePath, "", "", releasePathBoshCache)
	if !assert.NoError(err) {
		return
	}

	tempDir, err := ioutil.TempDir("", "fissile-test-compile-chroot")
	if !assert.NoError(err) {
		return
	}
	defer os.RemoveAll(tempDir)

	c, err := NewChrootCompilator(tempDir, "", "repo", "ubuntu", "0", ui)
	assert.NoError(err)

	err = c.Compile(2, []*model.Release{release}, nil)
	assert.NoError(err, stderr.String())
}
