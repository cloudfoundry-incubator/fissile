package compilator

import (
	"bytes"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/SUSE/fissile/model"
	"github.com/SUSE/termui"
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

func TestCompilePackageInMountNS(t *testing.T) {
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

	tempDir, err := ioutil.TempDir("", "fissile-test-compile-mount-ns")
	if !assert.NoError(err) {
		return
	}
	defer os.RemoveAll(tempDir)

	c, err := NewMountNSCompilator(tempDir, "", "repo", "linux", "0", ui, nil)
	assert.NoError(err)

	err = c.Compile(2, []*model.Release{release}, nil, false)
	assert.NoError(err, stderr.String())
}
